package anthropic

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/2144983846/aperture/internal/config"
	"github.com/2144983846/aperture/internal/provider"
)

const anthropicVersion = "2023-06-01"

type Adapter struct {
	client  *http.Client
	apiKey  string
	baseURL string
	models  []provider.ModelInfo
}

func New(cfg config.ProviderConfig) (*Adapter, error) {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://api.anthropic.com"
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	a := &Adapter{
		client: &http.Client{
			Timeout: 120 * time.Second,
		},
		apiKey:  cfg.APIKey,
		baseURL: baseURL,
	}

	for _, m := range cfg.Models {
		a.models = append(a.models, provider.ModelInfo{
			ID:              m.ID,
			ProviderID:      cfg.ID,
			CostPer1KInput:  m.CostPer1KInput,
			CostPer1KOutput: m.CostPer1KOutput,
			MaxTokens:       m.MaxTokens,
			Capabilities:    []string{"chat"},
		})
	}

	return a, nil
}

func (a *Adapter) ID() string { return "anthropic" }

type anthropicRequest struct {
	Model       string              `json:"model"`
	Messages    []anthropicMessage  `json:"messages"`
	System      string              `json:"system,omitempty"`
	MaxTokens   int                 `json:"max_tokens"`
	Temperature *float64            `json:"temperature,omitempty"`
	TopP        *float64            `json:"top_p,omitempty"`
	Stream      bool                `json:"stream"`
}

type anthropicMessage struct {
	Role    string              `json:"role"`
	Content []anthropicContent  `json:"content"`
}

type anthropicContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type anthropicResponse struct {
	ID      string             `json:"id"`
	Model   string             `json:"model"`
	Content []anthropicContent `json:"content"`
	Usage   anthropicUsage     `json:"usage"`
}

type anthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type anthropicStreamEvent struct {
	Type  string            `json:"type"`
	Delta *anthropicDelta   `json:"delta,omitempty"`
	Usage *anthropicUsage   `json:"usage,omitempty"`
}

type anthropicDelta struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func (a *Adapter) ChatCompletion(ctx context.Context, req *provider.ChatRequest) (*provider.ChatResponse, error) {
	ar := a.translateRequest(req)
	ar.Stream = false

	body, err := json.Marshal(ar)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, a.baseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", a.apiKey)
	httpReq.Header.Set("anthropic-version", anthropicVersion)

	resp, err := a.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		errBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("anthropic error %d: %s", resp.StatusCode, string(errBody))
	}

	var arResp anthropicResponse
	if err := json.NewDecoder(resp.Body).Decode(&arResp); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	return a.translateResponse(&arResp), nil
}

func (a *Adapter) ChatCompletionStream(ctx context.Context, req *provider.ChatRequest) (<-chan provider.StreamChunk, error) {
	ar := a.translateRequest(req)
	ar.Stream = true

	body, err := json.Marshal(ar)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, a.baseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", a.apiKey)
	httpReq.Header.Set("anthropic-version", anthropicVersion)

	resp, err := a.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		errBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("anthropic error %d: %s", resp.StatusCode, string(errBody))
	}

	ch := make(chan provider.StreamChunk, 10)
	go a.readStream(ctx, resp, ch)
	return ch, nil
}

func (a *Adapter) readStream(ctx context.Context, resp *http.Response, ch chan<- provider.StreamChunk) {
	defer resp.Body.Close()
	defer close(ch)

	scanner := bufio.NewScanner(resp.Body)
	var contentBuilder strings.Builder
	chunkIdx := 0

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
		}

		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		payload := strings.TrimPrefix(line, "data: ")

		var event anthropicStreamEvent
		if err := json.Unmarshal([]byte(payload), &event); err != nil {
			continue
		}

		switch event.Type {
		case "content_block_delta":
			if event.Delta != nil && event.Delta.Type == "text_delta" {
				text := event.Delta.Text
				contentBuilder.WriteString(text)

				ch <- provider.StreamChunk{
					ID:    fmt.Sprintf("chatcmpl-%d", chunkIdx),
					Model: "claude",
					Choices: []provider.Choice{{
						Index: 0,
						Delta: &provider.Message{Role: "assistant", Content: text},
					}},
				}
				chunkIdx++
			}
		case "message_delta":
			if event.Usage != nil {
				ch <- provider.StreamChunk{
					Model: "claude",
					Usage: &provider.Usage{
						CompletionTokens: event.Usage.OutputTokens,
					},
				}
			}
		}
	}
}

func (a *Adapter) ListModels(ctx context.Context) ([]provider.ModelInfo, error) {
	return a.models, nil
}

func (a *Adapter) Health(ctx context.Context) error {
	url := a.baseURL + "/v1/messages"
	httpReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	httpReq.Header.Set("x-api-key", a.apiKey)
	httpReq.Header.Set("anthropic-version", anthropicVersion)

	resp, err := a.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 405 || resp.StatusCode == 404 {
		return nil
	}
	if resp.StatusCode >= 500 {
		return fmt.Errorf("anthropic returned status %d", resp.StatusCode)
	}
	return nil
}

func (a *Adapter) translateRequest(req *provider.ChatRequest) *anthropicRequest {
	ar := &anthropicRequest{
		Model:     req.Model,
		MaxTokens: 4096,
	}

	if req.MaxTokens != nil {
		ar.MaxTokens = *req.MaxTokens
	}
	if req.Temperature != nil {
		ar.Temperature = req.Temperature
	}
	if req.TopP != nil {
		ar.TopP = req.TopP
	}

	for _, msg := range req.Messages {
		if msg.Role == "system" {
			ar.System = msg.Content
			continue
		}

		role := msg.Role
		if role == "assistant" {
			role = "assistant"
		} else {
			role = "user"
		}

		ar.Messages = append(ar.Messages, anthropicMessage{
			Role:    role,
			Content: []anthropicContent{{Type: "text", Text: msg.Content}},
		})
	}

	return ar
}

func (a *Adapter) translateResponse(ar *anthropicResponse) *provider.ChatResponse {
	var text string
	for _, c := range ar.Content {
		if c.Type == "text" {
			text += c.Text
		}
	}

	finish := "stop"
	return &provider.ChatResponse{
		ID:      ar.ID,
		Object:  "chat.completion",
		Model:   ar.Model,
		Created: 0,
		Choices: []provider.Choice{{
			Index:   0,
			Message: &provider.Message{Role: "assistant", Content: text},
			FinishReason: &finish,
		}},
		Usage: provider.Usage{
			PromptTokens:     ar.Usage.InputTokens,
			CompletionTokens: ar.Usage.OutputTokens,
			TotalTokens:      ar.Usage.InputTokens + ar.Usage.OutputTokens,
		},
	}
}
