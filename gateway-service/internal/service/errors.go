package service

import "errors"

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrDuplicateEntity    = errors.New("entity already exists")
	ErrUniversityPending  = errors.New("university account is pending approval")
	ErrUniversityBlocked  = errors.New("university account is blocked")
	ErrUniversityInactive = errors.New("university account is not active")
	ErrDiplomaNotFound    = errors.New("diploma not found")
	ErrShareLinkExpired   = errors.New("share link expired")
	ErrInvalidShareToken  = errors.New("invalid share token")
)
