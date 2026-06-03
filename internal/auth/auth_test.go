package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStaticKeyStore_Validate_Active(t *testing.T) {
	keys := []*APIKey{
		{ID: "sk-abc", Name: "test-key", IsActive: true},
	}
	store := NewStaticKeyStore(keys)

	k, err := store.Validate("sk-abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if k == nil {
		t.Fatal("expected non-nil key")
	}
	if k.Name != "test-key" {
		t.Errorf("expected name 'test-key', got %q", k.Name)
	}
}

func TestStaticKeyStore_Validate_Inactive(t *testing.T) {
	keys := []*APIKey{
		{ID: "sk-inactive", Name: "inactive", IsActive: false},
	}
	store := NewStaticKeyStore(keys)

	k, err := store.Validate("sk-inactive")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if k != nil {
		t.Error("expected nil for inactive key")
	}
}

func TestStaticKeyStore_Validate_NotFound(t *testing.T) {
	store := NewStaticKeyStore(nil)

	k, err := store.Validate("no-such-key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if k != nil {
		t.Error("expected nil for missing key")
	}
}

func TestStaticKeyStore_EmptyStore(t *testing.T) {
	store := NewStaticKeyStore([]*APIKey{})

	k, err := store.Validate("any-key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if k != nil {
		t.Error("expected nil for empty store")
	}
}

func TestRateLimiter_Allow(t *testing.T) {
	rate := 3
	rl := NewRateLimiter(rate)

	// First 3 requests should be allowed
	for i := 0; i < rate; i++ {
		if !rl.Allow("key-1") {
			t.Errorf("request %d should be allowed", i+1)
		}
	}

	// 4th request should be denied
	if rl.Allow("key-1") {
		t.Error("request should be rate-limited")
	}
}

func TestRateLimiter_IndependentKeys(t *testing.T) {
	rl := NewRateLimiter(2)

	// Consume key-1's quota
	rl.Allow("key-1")
	rl.Allow("key-1")

	// key-2 should still have its full quota
	if !rl.Allow("key-2") {
		t.Error("key-2 should be allowed (independent buckets)")
	}
}

func TestExtractKey(t *testing.T) {
	tests := []struct {
		name    string
		headers map[string]string
		want    string
	}{
		{
			name:    "bearer token",
			headers: map[string]string{"Authorization": "Bearer sk-test-123"},
			want:    "sk-test-123",
		},
		{
			name:    "raw auth header",
			headers: map[string]string{"Authorization": "sk-direct"},
			want:    "sk-direct",
		},
		{
			name:    "x-api-key header",
			headers: map[string]string{"X-API-Key": "sk-alt"},
			want:    "sk-alt",
		},
		{
			name:    "no headers",
			headers: map[string]string{},
			want:    "",
		},
		{
			name:    "empty authorization",
			headers: map[string]string{"Authorization": ""},
			want:    "",
		},
		{
			name:    "bearer with extra spaces",
			headers: map[string]string{"Authorization": "Bearer sk-space-test"},
			want:    "sk-space-test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}
			got := extractKey(req)
			if got != tt.want {
				t.Errorf("extractKey() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGetAPIKey_NilContext(t *testing.T) {
	ctx := context.Background()
	k := GetAPIKey(ctx)
	if k != nil {
		t.Error("expected nil for empty context")
	}
}

func TestGetAPIKey_WithKey(t *testing.T) {
	expected := &APIKey{ID: "sk-123", Name: "my-key"}
	ctx := context.WithValue(context.Background(), ctxKeyAPIKey, expected)

	k := GetAPIKey(ctx)
	if k == nil {
		t.Fatal("expected non-nil key")
	}
	if k.ID != "sk-123" {
		t.Errorf("expected ID 'sk-123', got %q", k.ID)
	}
}

func TestNewRateLimiter(t *testing.T) {
	rl := NewRateLimiter(60)
	if rl == nil {
		t.Fatal("expected non-nil rate limiter")
	}
	if rl.rate != 60 {
		t.Errorf("expected rate 60, got %d", rl.rate)
	}
}

func TestNewStaticKeyStore(t *testing.T) {
	keys := []*APIKey{
		{ID: "k1", Name: "alpha", IsActive: true},
		{ID: "k2", Name: "beta", IsActive: true},
	}
	store := NewStaticKeyStore(keys)

	// Verify both keys are accessible
	k1, _ := store.Validate("k1")
	if k1 == nil || k1.Name != "alpha" {
		t.Error("k1 not found correctly")
	}
	k2, _ := store.Validate("k2")
	if k2 == nil || k2.Name != "beta" {
		t.Error("k2 not found correctly")
	}
}

func TestMiddleware_Authenticate_MissingKey(t *testing.T) {
	store := NewStaticKeyStore(nil)
	m := NewMiddleware(store, 100)

	handler := m.Authenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestMiddleware_Authenticate_InvalidKey(t *testing.T) {
	store := NewStaticKeyStore([]*APIKey{
		{ID: "valid-key", IsActive: true},
	})
	m := NewMiddleware(store, 100)

	handler := m.Authenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called for invalid key")
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer invalid-key")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestMiddleware_Authenticate_ValidKey(t *testing.T) {
	store := NewStaticKeyStore([]*APIKey{
		{ID: "sk-valid", Name: "valid", IsActive: true, RateLimitRPM: 100},
	})
	m := NewMiddleware(store, 100)

	called := false
	handler := m.Authenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		key := GetAPIKey(r.Context())
		if key == nil {
			t.Error("expected API key in context")
		}
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer sk-valid")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Error("handler should have been called")
	}
}

func TestAPIKey_Fields(t *testing.T) {
	k := APIKey{
		ID:            "sk-1",
		Name:          "prod",
		ProjectID:     "proj-1",
		RateLimitRPM:  60,
		BudgetUSD:     100.0,
		AllowedModels: []string{"gpt-4o-mini", "claude-3-haiku"},
		IsActive:      true,
	}

	if k.ID != "sk-1" {
		t.Errorf("unexpected ID")
	}
	if k.RateLimitRPM != 60 {
		t.Errorf("unexpected rate limit")
	}
	if k.BudgetUSD != 100.0 {
		t.Errorf("unexpected budget")
	}
	if len(k.AllowedModels) != 2 {
		t.Errorf("expected 2 allowed models")
	}
}
