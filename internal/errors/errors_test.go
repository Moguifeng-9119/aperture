package errors

import (
	"errors"
	"net/http"
	"testing"
)

func TestStatusCode(t *testing.T) {
	tests := []struct {
		code   Code
		status int
	}{
		{ErrAuthMissing, http.StatusUnauthorized},
		{ErrAuthInvalid, http.StatusUnauthorized},
		{ErrRateLimited, http.StatusTooManyRequests},
		{ErrForbidden, http.StatusForbidden},
		{ErrBadRequest, http.StatusBadRequest},
		{ErrInvalidModel, http.StatusBadRequest},
		{ErrProviderNotFound, http.StatusNotFound},
		{ErrProviderTimeout, http.StatusGatewayTimeout},
		{ErrNoRuleMatch, http.StatusInternalServerError},
		{ErrInternal, http.StatusInternalServerError},
		{Code("UNKNOWN"), http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(string(tt.code), func(t *testing.T) {
			if got := StatusCode(tt.code); got != tt.status {
				t.Errorf("StatusCode(%s) = %d, want %d", tt.code, got, tt.status)
			}
		})
	}
}

func TestNew(t *testing.T) {
	e := New(ErrInternal, "something broke")
	if e.Code != ErrInternal {
		t.Errorf("expected code %s, got %s", ErrInternal, e.Code)
	}
	if e.Message != "something broke" {
		t.Errorf("expected message, got %q", e.Message)
	}
	if e.HTTPStatus != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", e.HTTPStatus)
	}
}

func TestWrap(t *testing.T) {
	inner := errors.New("connection refused")
	e := Wrap(ErrProviderError, "upstream failed", inner)
	if !errors.Is(e, inner) {
		t.Error("expected errors.Is to unwrap to inner error")
	}
	if e.Err != inner {
		t.Error("expected Err to be inner error")
	}
}

func TestError(t *testing.T) {
	e := New(ErrAuthMissing, "no API key provided")
	if e.Error() != "[AUTH_MISSING] no API key provided" {
		t.Errorf("unexpected error string: %q", e.Error())
	}

	w := Wrap(ErrProviderTimeout, "request timed out", errors.New("timeout"))
	if w.Error() != "[PROVIDER_TIMEOUT] request timed out: timeout" {
		t.Errorf("unexpected error string: %q", w.Error())
	}
}

func TestIsApertureError(t *testing.T) {
	e := New(ErrBadRequest, "bad input")
	ae, ok := IsApertureError(e)
	if !ok {
		t.Error("expected IsApertureError to return true")
	}
	if ae.Code != ErrBadRequest {
		t.Errorf("unexpected code: %s", ae.Code)
	}

	_, ok = IsApertureError(errors.New("plain error"))
	if ok {
		t.Error("expected IsApertureError to return false for plain error")
	}
}
