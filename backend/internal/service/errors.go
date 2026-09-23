package service

import (
	"errors"
	"fmt"
	"time"
)

// ErrInvalidRequest is returned for bad query parameters.
var ErrInvalidRequest = errors.New("invalid request")

// InvalidRequestError carries a client-facing validation message.
type InvalidRequestError struct {
	Message string
}

func (e *InvalidRequestError) Error() string {
	return e.Message
}

func (e *InvalidRequestError) Is(target error) bool {
	return target == ErrInvalidRequest
}

func NewInvalidRequestError(msg string) error {
	return &InvalidRequestError{Message: msg}
}

// ErrRateLimited is returned when force refresh is requested too frequently.
var ErrRateLimited = errors.New("rate limited")

// RateLimitedError asks the client to wait before refreshing again.
type RateLimitedError struct {
	RetryAfter time.Duration
	Message    string
}

func (e *RateLimitedError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return "refresh rate limited"
}

func (e *RateLimitedError) Is(target error) bool {
	return target == ErrRateLimited
}

func NewRateLimitedError(retryAfter time.Duration) error {
	secs := int(retryAfter.Seconds())
	if secs < 1 {
		secs = 1
	}
	return &RateLimitedError{
		RetryAfter: retryAfter,
		Message:    fmt.Sprintf("refresh limited to once per 5 minutes; retry after %ds", secs),
	}
}

// ErrUnknownAccount is returned when an account name or id is not configured.
var ErrUnknownAccount = errors.New("unknown account")

// UnknownAccountError identifies a request for an account that does not exist.
type UnknownAccountError struct {
	Account string
}

func (e *UnknownAccountError) Error() string {
	return fmt.Sprintf("unknown account %q", e.Account)
}

func (e *UnknownAccountError) Is(target error) bool {
	return target == ErrUnknownAccount
}

// NewUnknownAccountError wraps ErrUnknownAccount for a specific account identifier.
func NewUnknownAccountError(account string) error {
	return &UnknownAccountError{Account: account}
}
