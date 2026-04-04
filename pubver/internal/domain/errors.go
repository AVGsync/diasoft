package domain

import "errors"

var (
	ErrInvalidPayload                    = errors.New("invalid payload")
	ErrInvalidInput                      = errors.New("invalid input")
	ErrDiplomaHashClaimMismatch          = errors.New("diploma_hash claim mismatch")
	ErrSubClaimMismatch                  = errors.New("sub claim mismatch")
	ErrUniversityVerificationKeyNotFound = errors.New("university verification key not found")
)
