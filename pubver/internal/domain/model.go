package domain

import "time"

type DiplomaStatus string

const (
	DiplomaStatusActive   DiplomaStatus = "active"
	DiplomaStatusRevoked  DiplomaStatus = "revoked"
	DiplomaStatusNotFound DiplomaStatus = "not_found"
)

type DiplomaRecord struct {
	Hash          string
	DiplomaNumber string
	Status        DiplomaStatus
	University    University
	GraduateYear  *int
	Specialty     *string
	Degree        *string
	Faculty       *string
	QRPayload     *string
	RevokedAt     *time.Time
}

type University struct {
	Code string
	Name string
}

type UniversityVerificationKey struct {
	PublicKey string
}

type VerifyResponse struct {
	Valid         bool          `json:"valid"`
	Status        DiplomaStatus `json:"status"`
	Hash          string        `json:"hash,omitempty"`
	DiplomaNumber string        `json:"diploma_number,omitempty"`
	University    string        `json:"university,omitempty"`
	VUZCode       string        `json:"vuz_code,omitempty"`
	Year          *int          `json:"year,omitempty"`
	Specialty     *string       `json:"specialty,omitempty"`
	Degree        *string       `json:"degree,omitempty"`
	Faculty       *string       `json:"faculty,omitempty"`
	RevokedAt     *time.Time    `json:"revoked_at,omitempty"`
}

type SearchResponse struct {
	Valid      bool          `json:"valid"`
	Status     DiplomaStatus `json:"status"`
	University string        `json:"university,omitempty"`
	VUZCode    string        `json:"vuz_code,omitempty"`
	Year       *int          `json:"year,omitempty"`
	Specialty  *string       `json:"specialty,omitempty"`
	Degree     *string       `json:"degree,omitempty"`
	Faculty    *string       `json:"faculty,omitempty"`
}
