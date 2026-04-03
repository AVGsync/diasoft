package domain

import "errors"

var (
	ErrInvalidPayload = errors.New("invalid payload")
	ErrInvalidInput   = errors.New("invalid input")
)
