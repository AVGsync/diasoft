package model

import "time"

const (
	RoleUniversity = "university"
	RoleAdmin      = "admin"

	UniversityStatusPending = "pending"
	UniversityStatusActive  = "active"
	UniversityStatusBlocked = "blocked"

	BatchStatusProcessing = "processing"
	BatchStatusCompleted  = "completed"
	BatchStatusFailed     = "failed"

	RecordStatusPending   = "pending"
	RecordStatusProcessed = "processed"
	RecordStatusError     = "error"

	DiplomaStatusActive  = "active"
	DiplomaStatusRevoked = "revoked"
)

type RegisterUniversityRequest struct {
	Name     string `json:"name" validate:"required,min=3,max=255"`
	VuzCode  string `json:"vuz_code" validate:"required,alphanum,len=8"`
	INN      string `json:"inn" validate:"required,min=10,max=12"`
	OGRN     string `json:"ogrn" validate:"required,min=13,max=15"`
	Email    string `json:"email" validate:"required,email,max=255"`
	Password string `json:"password" validate:"required,min=8,max=128"`
}

type RegisterUniversityResponse struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

type LoginRequest struct {
	Email    string `json:"email" validate:"required,email,max=255"`
	Password string `json:"password" validate:"required,min=8,max=128"`
}

type AuthResponse struct {
	AccessToken string    `json:"access_token"`
	TokenType   string    `json:"token_type"`
	ExpiresAt   time.Time `json:"expires_at"`
	Role        string    `json:"role"`
	Status      string    `json:"status"`
	VUZID       string    `json:"vuz_id,omitempty"`
	Email       string    `json:"email"`
}

type University struct {
	ID           string
	Name         string
	VuzCode      string
	INN          string
	OGRN         string
	Email        string
	PasswordHash string
	PublicKey    *string
	Status       string
	CreatedAt    time.Time
}

type UpdateUniversityStatusRequest struct {
	Status string `json:"status" validate:"required,oneof=pending active blocked"`
}

type UniversityAdminResponse struct {
	ID           string    `json:"id"`
	VuzCode      string    `json:"vuz_code"`
	Name         string    `json:"name"`
	INN          string    `json:"inn"`
	OGRN         string    `json:"ogrn"`
	Email        string    `json:"email"`
	Status       string    `json:"status"`
	HasPublicKey bool      `json:"has_public_key"`
	CreatedAt    time.Time `json:"created_at"`
}

type UniversitySigningKey struct {
	VUZID                string
	EncryptedPrivateKey  string
	KeyAlgorithm         string
	EncryptionAlgorithm  string
	PublicKeyFingerprint string
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type PlatformAdmin struct {
	ID           string
	Email        string
	PasswordHash string
	CreatedAt    time.Time
}

type APIKey struct {
	ID        string    `json:"id"`
	VUZID     string    `json:"vuz_id"`
	Name      *string   `json:"name,omitempty"`
	KeyHash   string    `json:"-"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
}

type APIKeySummary struct {
	ID        string    `json:"id"`
	Name      *string   `json:"name,omitempty"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
}

type CreateAPIKeyRequest struct {
	Name *string `json:"name,omitempty" validate:"omitempty,min=3,max=100"`
}

type UpsertSigningKeyRequest struct {
	PrivateKeyPEM string `json:"private_key_pem" validate:"required,min=32"`
}

type CreateAPIKeyResponse struct {
	ID        string    `json:"id"`
	Name      *string   `json:"name,omitempty"`
	APIKey    string    `json:"api_key"`
	CreatedAt time.Time `json:"created_at"`
}

type SigningKeyStatusResponse struct {
	Configured           bool      `json:"configured"`
	KeyAlgorithm         string    `json:"key_algorithm"`
	EncryptionAlgorithm  string    `json:"encryption_algorithm"`
	PublicKeyFingerprint string    `json:"public_key_fingerprint"`
	UpdatedAt            time.Time `json:"updated_at"`
}

type Batch struct {
	ID               string     `json:"id"`
	VUZID            string     `json:"vuz_id"`
	Status           string     `json:"status"`
	TotalRecords     int        `json:"total_records"`
	ProcessedRecords int        `json:"processed_records"`
	FailedRecords    int        `json:"failed_records"`
	CreatedAt        time.Time  `json:"created_at"`
	CompletedAt      *time.Time `json:"completed_at,omitempty"`
}

type BatchUploadResponse struct {
	BatchID string `json:"batch_id"`
	Status  string `json:"status"`
}

type DiplomaUploadRequest struct {
	Diplomas []DiplomaUploadRecord `json:"diplomas" validate:"required,min=1,dive"`
}

type DiplomaUploadRecord struct {
	FullName      string `json:"full_name" validate:"required,min=3,max=255"`
	DiplomaNumber string `json:"diploma_number" validate:"required,min=3,max=50"`
	Specialty     string `json:"specialty" validate:"required,min=2,max=255"`
	Degree        string `json:"degree" validate:"required,oneof=Бакалавр Магистр Специалист"`
	Faculty       string `json:"faculty" validate:"required,min=2,max=255"`
	Year          int    `json:"year" validate:"required,gte=1900,lte=2100"`
	RawCSVRow     string `json:"-"`
}

type BatchDownloadRow struct {
	RecordIndex   int
	DiplomaHash   string
	FullName      string
	DiplomaNumber string
	Specialty     string
	Degree        string
	Faculty       string
	Year          int
	QRPayload     string
	Status        string
	Error         *string
}

type AdminStats struct {
	TotalUniversities   int64 `json:"total_universities"`
	PendingUniversities int64 `json:"pending_universities"`
	ActiveUniversities  int64 `json:"active_universities"`
	BlockedUniversities int64 `json:"blocked_universities"`
	TotalBatches        int64 `json:"total_batches"`
	TotalDiplomas       int64 `json:"total_diplomas"`
	RevokedDiplomas     int64 `json:"revoked_diplomas"`
}

type StudentSearchResult struct {
	DiplomaHash    string    `json:"diploma_hash"`
	DiplomaNumber  string    `json:"diploma_number"`
	FullName       string    `json:"full_name"`
	Specialty      string    `json:"specialty"`
	Degree         string    `json:"degree"`
	Faculty        string    `json:"faculty"`
	Year           int       `json:"year"`
	UniversityID   string    `json:"university_id"`
	UniversityName string    `json:"university_name"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
}

type CreateShareLinkRequest struct {
	DiplomaHash string `json:"diploma_hash" validate:"required,len=64"`
	TTLHours    *int   `json:"ttl_hours,omitempty" validate:"omitempty,gte=1,lte=720"`
}

type ShareLink struct {
	ID          string
	DiplomaHash string
	Token       string
	ExpiresAt   time.Time
	UsedCount   int
	CreatedAt   time.Time
}

type ShareLinkResponse struct {
	ShareURL  string    `json:"share_url"`
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

type SharedDiplomaResponse struct {
	DiplomaHash    string    `json:"diploma_hash"`
	DiplomaNumber  string    `json:"diploma_number"`
	FullName       string    `json:"full_name"`
	Specialty      string    `json:"specialty"`
	Degree         string    `json:"degree"`
	Faculty        string    `json:"faculty"`
	Year           int       `json:"year"`
	UniversityID   string    `json:"university_id"`
	UniversityName string    `json:"university_name"`
	Status         string    `json:"status"`
	ExpiresAt      time.Time `json:"expires_at"`
}
