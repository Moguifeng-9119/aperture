package pipeline

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/2144983846/aperture/internal/analytics"
	"github.com/2144983846/aperture/internal/auth"
	"github.com/2144983846/aperture/internal/conversation"
	"github.com/2144983846/aperture/internal/provider"
	"github.com/2144983846/aperture/internal/router"
	"github.com/2144983846/aperture/internal/router/strategy"
)

type Pipeline struct {
	router    *router.Router
	registry  *provider.Registry
	convStore *conversation.Store
	recorder  *analytics.Recorder
}

func New(r *router.Router, reg *provider.Registry, store *conversation.Store, recorder *analytics.Recorder) *Pipeline {
	return &Pipeline{
		router:    r,
		registry:  reg,
		convStore: store,
		recorder:  recorder,
	}
}

type Request struct {
	Model           string              `json:"model"`
	Messages        []provider.Message  `json:"messages"`
	Stream          bool                `json:"stream"`
	Temperature     *float64            `json:"temperature,omitempty"`
	MaxTokens       *int                `json:"max_tokens,omitempty"`
	ConversationID  string              `json:"-"`
	UserID          string              `json:"-"`
}

type Result struct {
	Response       *provider.ChatResponse
	StreamChunks   <-chan provider.StreamChunk
	IsStream       bool
	Decision       *strategy.Decision
	ConversationID string
}

func (p *Pipeline) Execute(ctx context.Context, req *Request) (*Result, error) {
	apiKey := auth.GetAPIKey(ctx)
	projectID := ""
	if apiKey != nil {
		projectID = apiKey.ProjectID
	}

	sess := p.convStore.GetOrCreate(req.ConversationID, projectID, req.UserID)

	convMsgs := p.convStore.GetMessages(sess.ID, 10)
	turnCount := len(convMsgs) / 2

	stratReq := &strategy.Request{
		Messages:       toStrategyMessages(req.Messages),
		ConversationID: sess.ID,
		TurnCount:      turnCount,
		ProjectID:      projectID,
	}

	decision, err := p.router.Classify(ctx, stratReq)
	if err != nil || decision == nil {
		slog.Error("routing failed", "error", err)
		return nil, fmt.Errorf("routing failed: %w", err)
	}

	targetProvider, err := p.registry.Get(decision.Provider)
	if err != nil {
		return nil, fmt.Errorf("provider %q not found: %w", decision.Provider, err)
	}

	chatReq := &provider.ChatRequest{
		Model:       decision.Model,
		Messages:    req.Messages,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
	}

	result := &Result{
		IsStream:       req.Stream,
		Decision:       decision,
		ConversationID: sess.ID,
	}

	start := time.Now()
	var tokensIn, tokensOut int
	var upstreamErr string
	var httpStatus int

	if req.Stream {
		chunks, err := targetProvider.ChatCompletionStream(ctx, chatReq)
		if err != nil {
			upstreamErr = err.Error()
			httpStatus = 502
		} else {
			result.StreamChunks = chunks
			httpStatus = 200
		}
	} else {
		resp, err := targetProvider.ChatCompletion(ctx, chatReq)
		if err != nil {
			upstreamErr = err.Error()
			httpStatus = 502
		} else {
			result.Response = resp
			httpStatus = 200
			tokensIn = resp.Usage.PromptTokens
			tokensOut = resp.Usage.CompletionTokens
		}
	}

	latency := time.Since(start)

	if p.recorder != nil {
		p.recorder.Record(decision, "rule", tokensIn, tokensOut, latency, httpStatus, upstreamErr, sess.ID, projectID)
	}

	allMsgs := make([]conversation.Message, 0, len(req.Messages)+1)
	for _, m := range req.Messages {
		allMsgs = append(allMsgs, conversation.Message{Role: m.Role, Content: m.Content})
	}
	if result.Response != nil {
		for _, c := range result.Response.Choices {
			if c.Message != nil {
				allMsgs = append(allMsgs, conversation.Message{
					Role:    c.Message.Role,
					Content: c.Message.Content,
				})
			}
		}
	}
	p.convStore.AddMessages(sess.ID, allMsgs)

	if upstreamErr != "" {
		return result, fmt.Errorf("dispatch: %s", upstreamErr)
	}

	return result, nil
}

func toStrategyMessages(msgs []provider.Message) []strategy.Message {
	result := make([]strategy.Message, len(msgs))
	for i, m := range msgs {
		result[i] = strategy.Message{Role: m.Role, Content: m.Content}
	}
	return result
}
