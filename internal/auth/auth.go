package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"
)

type KeyStore interface {
	Validate(key string) (*APIKey, error)
}

type APIKey struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	ProjectID      string    `json:"project_id"`
	RateLimitRPM   int       `json:"rate_limit_rpm"`
	BudgetUSD      float64   `json:"budget_monthly_usd"`
	AllowedModels  []string  `json:"allowed_models"`
	IsActive       bool      `json:"is_active"`
	LastUsedAt     time.Time `json:"last_used_at"`
}

type StaticKeyStore struct {
	keys map[string]*APIKey
}

func NewStaticKeyStore(keys []*APIKey) *StaticKeyStore {
	ks := &StaticKeyStore{keys: make(map[string]*APIKey)}
	for _, k := range keys {
		ks.keys[k.ID] = k
	}
	return ks
}

func (s *StaticKeyStore) Validate(key string) (*APIKey, error) {
	k, ok := s.keys[key]
	if !ok || !k.IsActive {
		return nil, nil
	}
	return k, nil
}

type RateLimiter struct {
	mu     sync.Mutex
	bucket map[string]*tokenBucket
	rate   int
}

type tokenBucket struct {
	tokens   float64
	lastFill time.Time
}

func NewRateLimiter(rate int) *RateLimiter {
	return &RateLimiter{
		bucket: make(map[string]*tokenBucket),
		rate:   rate,
	}
}

func (r *RateLimiter) Allow(keyID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	b, ok := r.bucket[keyID]
	if !ok {
		r.bucket[keyID] = &tokenBucket{tokens: float64(r.rate) - 1, lastFill: time.Now()}
		return true
	}

	now := time.Now()
	elapsed := now.Sub(b.lastFill).Minutes()
	b.tokens += elapsed * float64(r.rate)
	if b.tokens > float64(r.rate) {
		b.tokens = float64(r.rate)
	}
	b.lastFill = now

	if b.tokens >= 1 {
		b.tokens--
		return true
	}
	return false
}

type Middleware struct {
	keys   KeyStore
	limiter *RateLimiter
}

func NewMiddleware(keys KeyStore, rateLimitRPM int) *Middleware {
	return &Middleware{
		keys:   keys,
		limiter: NewRateLimiter(rateLimitRPM),
	}
}

func (m *Middleware) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := extractKey(r)
		if key == "" {
			writeAuthError(w, "missing API key")
			return
		}

		apiKey, err := m.keys.Validate(key)
		if err != nil || apiKey == nil {
			writeAuthError(w, "invalid API key")
			return
		}

		if !m.limiter.Allow(apiKey.ID) {
			writeAuthError(w, "rate limit exceeded")
			return
		}

		ctx := context.WithValue(r.Context(), ctxKeyAPIKey, apiKey)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func extractKey(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if auth != "" {
		if strings.HasPrefix(auth, "Bearer ") {
			return strings.TrimPrefix(auth, "Bearer ")
		}
		return auth
	}

	if key := r.Header.Get("X-API-Key"); key != "" {
		return key
	}

	return ""
}

type contextKey string

var ctxKeyAPIKey contextKey = "api_key"

func GetAPIKey(ctx context.Context) *APIKey {
	if k, ok := ctx.Value(ctxKeyAPIKey).(*APIKey); ok {
		return k
	}
	return nil
}

func writeAuthError(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]any{
			"message": msg,
			"type": "authentication_error",
		},
	})
}
