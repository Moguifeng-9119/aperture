package groq

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/2144983846/aperture/internal/config"
	"github.com/2144983846/aperture/internal/provider"
)

type Adapter struct {
	client  *http.Client
	apiKey  string
	baseURL string
	models  []provider.ModelInfo
}

func New(cfg config.ProviderConfig) (*Adapter, error) {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://api.groq.com/openai/v1"
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	a := &Adapter{
		client:  &http.Client{},
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

func (a *Adapter) ID() string { return "groq" }

func (a *Adapter) ChatCompletion(ctx context.Context, req *provider.ChatRequest) (*provider.ChatResponse, error) {
	url := a.baseURL + "/chat/completions"

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+a.apiKey)

	resp, err := a.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		errBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("groq error %d: %s", resp.StatusCode, string(errBody))
	}

	var chatResp provider.ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	return &chatResp, nil
}

func (a *Adapter) ChatCompletionStream(ctx context.Context, req *provider.ChatRequest) (<-chan provider.StreamChunk, error) {
	req.Stream = true
	url := a.baseURL + "/chat/completions"

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+a.apiKey)

	resp, err := a.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		errBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("groq error %d: %s", resp.StatusCode, string(errBody))
	}

	ch := make(chan provider.StreamChunk, 10)
	go a.readStream(ctx, resp, ch)
	return ch, nil
}

func (a *Adapter) readStream(ctx context.Context, resp *http.Response, ch chan<- provider.StreamChunk) {
	defer resp.Body.Close()
	defer close(ch)

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
		}

		line := scanner.Text()
		if line == "" || line == "data: [DONE]" {
			continue
		}
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		payload := strings.TrimPrefix(line, "data: ")
		var chunk provider.StreamChunk
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			continue
		}
		ch <- chunk
	}
}

func (a *Adapter) ListModels(ctx context.Context) ([]provider.ModelInfo, error) {
	return a.models, nil
}

func (a *Adapter) Health(ctx context.Context) error {
	url := a.baseURL + "/models"
	httpReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	httpReq.Header.Set("Authorization", "Bearer "+a.apiKey)

	resp, err := a.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		return fmt.Errorf("groq returned status %d", resp.StatusCode)
	}
	return nil
}
