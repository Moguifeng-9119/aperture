package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
)

type Proxy struct {
	APIKey  string
	BaseURL string
	Models  []Model
}

type OpenAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type OpenAIRequest struct {
	Model    string          `json:"model"`
	Messages []OpenAIMessage `json:"messages"`
	Stream   bool            `json:"stream"`
}

type AnthropicContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type AnthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type AnthropicRequest struct {
	Model     string             `json:"model"`
	Messages  []AnthropicMessage `json:"messages"`
	Stream    bool               `json:"stream"`
	MaxTokens int                `json:"max_tokens"`
}

func anthropicToOpenAI(ar *AnthropicRequest) *OpenAIRequest {
	messages := make([]OpenAIMessage, len(ar.Messages))
	for i, m := range ar.Messages {
		messages[i] = OpenAIMessage{Role: m.Role, Content: m.Content}
	}
	return &OpenAIRequest{
		Model:    ar.Model,
		Messages: messages,
		Stream:   ar.Stream,
	}
}

func openaiSSEToAnthropic(data []byte) []byte {
	if !bytes.HasPrefix(data, []byte(`data: `)) {
		return data
	}
	payload := bytes.TrimPrefix(data, []byte(`data: `))
	if bytes.Equal(payload, []byte("[DONE]")) {
		return data
	}
	var event struct {
		Choices []struct {
			Delta struct {
				Content          string `json:"content"`
				ReasoningContent string `json:"reasoning_content"`
			} `json:"delta"`
			FinishReason *string `json:"finish_reason"`
			Index        int     `json:"index"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(payload, &event); err != nil {
		return data
	}
	for _, choice := range event.Choices {
		delta := choice.Delta
		switch {
		case choice.FinishReason != nil && *choice.FinishReason == "stop":
			return []byte(`data: {"type":"message_stop"}`)
		case delta.ReasoningContent != "":
			evt := fmt.Sprintf(
				`data: {"type":"content_block_delta","index":%d,"delta":{"type":"thinking_delta","thinking":"%s"}}`,
				choice.Index, escapeJSON(delta.ReasoningContent))
			return []byte(evt)
		case delta.Content != "":
			evt := fmt.Sprintf(
				`data: {"type":"content_block_delta","index":%d,"delta":{"type":"text_delta","text":"%s"}}`,
				choice.Index, escapeJSON(delta.Content))
			return []byte(evt)
		}
	}
	return data
}

func openaiToAnthropic(body []byte) ([]byte, error) {
	var oai struct {
		Choices []struct {
			Message struct {
				Content          string `json:"content"`
				ReasoningContent string `json:"reasoning_content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &oai); err != nil {
		return nil, fmt.Errorf("parse openai response: %w", err)
	}
	if len(oai.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}
	content := oai.Choices[0].Message.Content
	if content == "" {
		content = oai.Choices[0].Message.ReasoningContent
	}
	anthropic := map[string]interface{}{
		"id":      "msg_1",
		"type":    "message",
		"role":    "assistant",
		"model":   "claude",
		"content": []map[string]interface{}{{"type": "text", "text": content}},
		"stop_reason": "end_turn",
		"usage": map[string]int{
			"input_tokens":  oai.Usage.PromptTokens,
			"output_tokens": oai.Usage.CompletionTokens,
		},
	}
	return json.Marshal(anthropic)
}

func (p *Proxy) Forward(w http.ResponseWriter, r *http.Request, oaiReq *OpenAIRequest) {
	body, err := json.Marshal(oaiReq)
	if err != nil {
		http.Error(w, `{"error":{"message":"marshal request failed"}}`, http.StatusInternalServerError)
		return
	}
	url := strings.TrimRight(p.BaseURL, "/") + "/chat/completions"
	slog.Info("forwarding", "url", url, "model", oaiReq.Model, "stream", oaiReq.Stream)
	upstream, err := http.NewRequestWithContext(r.Context(), "POST", url, bytes.NewReader(body))
	if err != nil {
		http.Error(w, `{"error":{"message":"create upstream request failed"}}`, http.StatusInternalServerError)
		return
	}
	upstream.Header.Set("Content-Type", "application/json")
	upstream.Header.Set("Authorization", "Bearer "+p.APIKey)
	resp, err := http.DefaultClient.Do(upstream)
	if err != nil {
		slog.Error("upstream failed", "error", err)
		http.Error(w, fmt.Sprintf(`{"error":{"message":"upstream request failed: %v"}}`, err), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	if oaiReq.Stream {
		p.streamResponse(w, resp)
	} else {
		p.nonStreamResponse(w, resp)
	}
}

func (p *Proxy) streamResponse(w http.ResponseWriter, resp *http.Response) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}
	w.Write([]byte("event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_1\",\"type\":\"message\",\"role\":\"assistant\",\"model\":\"claude\",\"content\":[],\"usage\":{\"input_tokens\":0,\"output_tokens\":0}}}\n\n"))
	flusher.Flush()
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		converted := openaiSSEToAnthropic([]byte(line))
		w.Write(converted)
		w.Write([]byte("\n\n"))
		flusher.Flush()
	}
	w.Write([]byte("event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"))
	flusher.Flush()
}

func (p *Proxy) nonStreamResponse(w http.ResponseWriter, resp *http.Response) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, `{"error":{"message":"read upstream response failed"}}`, http.StatusInternalServerError)
		return
	}
	if resp.StatusCode >= 400 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(resp.StatusCode)
		w.Write(body)
		return
	}
	converted, err := openaiToAnthropic(body)
	if err != nil {
		slog.Error("convert response failed", "error", err)
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(converted)
}

func escapeJSON(s string) string {
	encoded, _ := json.Marshal(s)
	return string(encoded[1 : len(encoded)-1])
}
