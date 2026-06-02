package ollama

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

type Adapter struct {
	client  *http.Client
	baseURL string
	models  []provider.ModelInfo
}

func New(cfg config.ProviderConfig) (*Adapter, error) {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	a := &Adapter{
		client: &http.Client{
			Timeout: 120 * time.Second,
		},
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

func (a *Adapter) ID() string { return "ollama" }

type ollamaRequest struct {
	Model    string           `json:"model"`
	Messages []ollamaMessage  `json:"messages"`
	Stream   bool             `json:"stream"`
	Options  map[string]any   `json:"options,omitempty"`
}

type ollamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaResponse struct {
	Model     string         `json:"model"`
	CreatedAt time.Time      `json:"created_at"`
	Message   ollamaMessage  `json:"message"`
	EvalCount int            `json:"eval_count,omitempty"`
	PromptEvalCount int      `json:"prompt_eval_count,omitempty"`
	Done      bool           `json:"done"`
}

func (a *Adapter) ChatCompletion(ctx context.Context, req *provider.ChatRequest) (*provider.ChatResponse, error) {
	or := a.translateRequest(req)
	or.Stream = false

	body, err := json.Marshal(or)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, a.baseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		errBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama error %d: %s", resp.StatusCode, string(errBody))
	}

	return a.readFullResponse(resp, req.Model)
}

func (a *Adapter) ChatCompletionStream(ctx context.Context, req *provider.ChatRequest) (<-chan provider.StreamChunk, error) {
	or := a.translateRequest(req)
	or.Stream = true

	body, err := json.Marshal(or)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, a.baseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		errBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama error %d: %s", resp.StatusCode, string(errBody))
	}

	ch := make(chan provider.StreamChunk, 10)
	go a.readStream(ctx, resp, ch)
	return ch, nil
}

func (a *Adapter) readFullResponse(resp *http.Response, modelName string) (*provider.ChatResponse, error) {
	var result ollamaResponse

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		var line ollamaResponse
		if err := json.Unmarshal(scanner.Bytes(), &line); err != nil {
			continue
		}
		if line.Message.Content != "" {
			result.Message.Content += line.Message.Content
		}
		result.EvalCount += line.EvalCount
		result.PromptEvalCount += line.PromptEvalCount
		if line.Done {
			result.Done = true
			break
		}
	}

	if !result.Done && result.Message.Content == "" {
		return nil, fmt.Errorf("ollama returned empty response")
	}

	finish := "stop"
	return &provider.ChatResponse{
		ID:      fmt.Sprintf("ollama-%d", time.Now().UnixNano()),
		Object:  "chat.completion",
		Model:   modelName,
		Choices: []provider.Choice{{
			Index:   0,
			Message: &provider.Message{Role: "assistant", Content: result.Message.Content},
			FinishReason: &finish,
		}},
		Usage: provider.Usage{
			PromptTokens:     result.PromptEvalCount,
			CompletionTokens: result.EvalCount,
			TotalTokens:      result.PromptEvalCount + result.EvalCount,
		},
	}, nil
}

func (a *Adapter) readStream(ctx context.Context, resp *http.Response, ch chan<- provider.StreamChunk) {
	defer resp.Body.Close()
	defer close(ch)

	scanner := bufio.NewScanner(resp.Body)
	chunkIdx := 0

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
		}

		var or ollamaResponse
		if err := json.Unmarshal(scanner.Bytes(), &or); err != nil {
			continue
		}

		if or.Message.Content != "" {
			ch <- provider.StreamChunk{
				ID:    fmt.Sprintf("chatcmpl-%d", chunkIdx),
				Model: or.Model,
				Choices: []provider.Choice{{
					Index: 0,
					Delta: &provider.Message{Role: "assistant", Content: or.Message.Content},
				}},
			}
			chunkIdx++
		}

		if or.Done {
			ch <- provider.StreamChunk{
				Model: or.Model,
				Usage: &provider.Usage{
					PromptTokens:     or.PromptEvalCount,
					CompletionTokens: or.EvalCount,
					TotalTokens:      or.PromptEvalCount + or.EvalCount,
				},
			}
			return
		}
	}
}

func (a *Adapter) ListModels(ctx context.Context) ([]provider.ModelInfo, error) {
	url := a.baseURL + "/api/tags"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return a.models, nil
	}

	resp, err := a.client.Do(httpReq)
	if err != nil {
		return a.models, nil
	}
	defer resp.Body.Close()

	var result struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return a.models, nil
	}

	var dynamicModels []provider.ModelInfo
	for _, m := range result.Models {
		dynamicModels = append(dynamicModels, provider.ModelInfo{
			ID:           m.Name,
			ProviderID:   "ollama",
			Capabilities: []string{"chat"},
		})
	}

	if len(dynamicModels) > 0 {
		return dynamicModels, nil
	}
	return a.models, nil
}

func (a *Adapter) Health(ctx context.Context) error {
	url := a.baseURL + "/api/tags"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}

	resp, err := a.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("ollama unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		return fmt.Errorf("ollama returned status %d", resp.StatusCode)
	}
	return nil
}

func (a *Adapter) translateRequest(req *provider.ChatRequest) *ollamaRequest {
	or := &ollamaRequest{
		Model:    req.Model,
		Messages: make([]ollamaMessage, len(req.Messages)),
		Options:  make(map[string]any),
	}

	for i, m := range req.Messages {
		or.Messages[i] = ollamaMessage{Role: m.Role, Content: m.Content}
	}

	if req.Temperature != nil {
		or.Options["temperature"] = *req.Temperature
	}
	if req.MaxTokens != nil {
		or.Options["num_predict"] = *req.MaxTokens
	}
	if req.TopP != nil {
		or.Options["top_p"] = *req.TopP
	}

	return or
}
