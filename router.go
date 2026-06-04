package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
)

type Router struct {
	providers       map[string]*Proxy
	rules           []Rule
	defaultModel    string
	defaultProvider string
}

func (rt *Router) Route(messages []AnthropicMessage) (*Proxy, string) {
	var userText string
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			userText = strings.ToLower(messages[i].Content)
			break
		}
	}
	for _, rule := range rt.rules {
		for _, kw := range rule.Keywords {
			if strings.Contains(userText, strings.ToLower(kw)) {
				p, ok := rt.providers[rule.Provider]
				if !ok {
					continue
				}
				slog.Info("rule matched", "rule", rule.Name, "keyword", kw, "model", rule.Model)
				return p, rule.Model
			}
		}
	}
	p := rt.providers[rt.defaultProvider]
	slog.Info("no rule matched, using default", "provider", rt.defaultProvider, "model", rt.defaultModel)
	return p, rt.defaultModel
}

func (rt *Router) handleMessages(w http.ResponseWriter, r *http.Request) {
	var ar AnthropicRequest
	if err := json.NewDecoder(r.Body).Decode(&ar); err != nil {
		http.Error(w, `{"error":{"message":"invalid request body","type":"aperture_error"}}`, http.StatusBadRequest)
		return
	}
	proxy, model := rt.Route(ar.Messages)
	oaiReq := anthropicToOpenAI(&ar)
	oaiReq.Model = model
	proxy.Forward(w, r, oaiReq)
}

func (rt *Router) handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	var oaiReq OpenAIRequest
	if err := json.NewDecoder(r.Body).Decode(&oaiReq); err != nil {
		http.Error(w, `{"error":{"message":"invalid request body"}}`, http.StatusBadRequest)
		return
	}
	msgs := make([]AnthropicMessage, len(oaiReq.Messages))
	for i, m := range oaiReq.Messages {
		msgs[i] = AnthropicMessage{Role: m.Role, Content: m.Content}
	}
	proxy, model := rt.Route(msgs)
	oaiReq.Model = model
	proxy.Forward(w, r, &oaiReq)
}
