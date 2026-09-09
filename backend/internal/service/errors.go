package service

import (
	"errors"
	"fmt"
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
