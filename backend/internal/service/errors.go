package service

import (
	"errors"
	"fmt"
)

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
