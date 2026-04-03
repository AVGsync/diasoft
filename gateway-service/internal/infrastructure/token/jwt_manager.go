package token

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Manager struct {
	accessSecret []byte
	shareSecret  []byte
	accessTTL    time.Duration
	shareTTL     time.Duration
}

type AccessClaims struct {
	VUZID  string `json:"vuz_id,omitempty"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	Status string `json:"status"`
	jwt.RegisteredClaims
}

type ShareClaims struct {
	DiplomaHash string `json:"diploma_hash"`
	Type        string `json:"type"`
	jwt.RegisteredClaims
}

func NewManager(accessSecret, shareSecret string, accessTTL, shareTTL time.Duration) *Manager {
	return &Manager{
		accessSecret: []byte(accessSecret),
		shareSecret:  []byte(shareSecret),
		accessTTL:    accessTTL,
		shareTTL:     shareTTL,
	}
}

func (m *Manager) IssueAccessToken(subject, vuzID, email, role, status string) (string, time.Time, error) {
	now := time.Now().UTC()
	expiresAt := now.Add(m.accessTTL)

	claims := &AccessClaims{
		VUZID:  vuzID,
		Email:  email,
		Role:   role,
		Status: status,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   subject,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(m.accessSecret)
	if err != nil {
		return "", time.Time{}, err
	}

	return signed, expiresAt, nil
}

func (m *Manager) ParseAccessToken(tokenString string) (*AccessClaims, error) {
	claims := &AccessClaims{}
	tokenValue, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("unexpected signing method")
		}
		return m.accessSecret, nil
	})
	if err != nil {
		return nil, err
	}
	if !tokenValue.Valid {
		return nil, errors.New("invalid access token")
	}

	return claims, nil
}

func (m *Manager) IssueShareToken(diplomaHash string, ttl time.Duration) (string, time.Time, error) {
	if ttl <= 0 {
		ttl = m.shareTTL
	}

	now := time.Now().UTC()
	expiresAt := now.Add(ttl)

	claims := &ShareClaims{
		DiplomaHash: diplomaHash,
		Type:        "share_link",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   diplomaHash,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(m.shareSecret)
	if err != nil {
		return "", time.Time{}, err
	}

	return signed, expiresAt, nil
}

func (m *Manager) ParseShareToken(tokenString string) (*ShareClaims, error) {
	claims := &ShareClaims{}
	tokenValue, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("unexpected signing method")
		}
		return m.shareSecret, nil
	})
	if err != nil {
		return nil, err
	}
	if !tokenValue.Valid {
		return nil, errors.New("invalid share token")
	}

	return claims, nil
}
