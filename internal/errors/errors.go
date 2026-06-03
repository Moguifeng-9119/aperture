package errors

import (
	"fmt"
	"net/http"
)

type Code string

const (
	// Authentication errors (4xx)
	ErrAuthMissing  Code = "AUTH_MISSING"
	ErrAuthInvalid  Code = "AUTH_INVALID"
	ErrAuthExpired  Code = "AUTH_EXPIRED"
	ErrRateLimited  Code = "RATE_LIMITED"
	ErrForbidden    Code = "FORBIDDEN"

	// Request errors (4xx)
	ErrBadRequest   Code = "BAD_REQUEST"
	ErrInvalidModel Code = "INVALID_MODEL"
	ErrBodyTooLarge Code = "BODY_TOO_LARGE"

	// Routing errors (5xx)
	ErrNoRuleMatch    Code = "NO_RULE_MATCH"
	ErrLowConfidence  Code = "LOW_CONFIDENCE"
	ErrStrategyFailed Code = "STRATEGY_FAILED"

	// Provider errors (5xx)
	ErrProviderNotFound Code = "PROVIDER_NOT_FOUND"
	ErrProviderTimeout  Code = "PROVIDER_TIMEOUT"
	ErrProviderError    Code = "PROVIDER_ERROR"
	ErrProviderRate     Code = "PROVIDER_RATE_LIMITED"

	// Internal errors (5xx)
	ErrInternal      Code = "INTERNAL_ERROR"
	ErrStoreFailure  Code = "STORE_FAILURE"
	ErrConfigInvalid Code = "CONFIG_INVALID"
)

type ApertureError struct {
	Code       Code   `json:"code"`
	Message    string `json:"message"`
	HTTPStatus int    `json:"-"`
	Err        error  `json:"-"`
}

func (e *ApertureError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e *ApertureError) Unwrap() error { return e.Err }

func New(code Code, msg string) *ApertureError {
	return &ApertureError{Code: code, Message: msg, HTTPStatus: StatusCode(code)}
}

func Wrap(code Code, msg string, err error) *ApertureError {
	return &ApertureError{Code: code, Message: msg, HTTPStatus: StatusCode(code), Err: err}
}

func StatusCode(c Code) int {
	switch c {
	case ErrAuthMissing, ErrAuthInvalid, ErrAuthExpired:
		return http.StatusUnauthorized
	case ErrRateLimited:
		return http.StatusTooManyRequests
	case ErrForbidden:
		return http.StatusForbidden
	case ErrBadRequest, ErrInvalidModel, ErrBodyTooLarge:
		return http.StatusBadRequest
	case ErrProviderNotFound:
		return http.StatusNotFound
	case ErrProviderTimeout:
		return http.StatusGatewayTimeout
	case ErrNoRuleMatch, ErrLowConfidence, ErrStrategyFailed,
		ErrProviderError, ErrProviderRate, ErrInternal, ErrStoreFailure, ErrConfigInvalid:
		return http.StatusInternalServerError
	default:
		return http.StatusInternalServerError
	}
}

func IsApertureError(err error) (*ApertureError, bool) {
	ae, ok := err.(*ApertureError)
	return ae, ok
}
