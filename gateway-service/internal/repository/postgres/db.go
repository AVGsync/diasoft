package postgres

import (
	"database/sql"

	_ "github.com/lib/pq"
)

type DB struct {
	config           *Config
	db               *sql.DB
	universityRepo   *UniversityRepository
	adminRepo        *AdminRepository
	apiKeyRepo       *APIKeyRepository
	diplomaRepo      *DiplomaRepository
	signingKeyRepo   *SigningKeyRepository
	qrPayloadDecoder QRPayloadDecoder
}

func New(config *Config) *DB {
	return &DB{config: config}
}

func (d *DB) Open() error {
	db, err := sql.Open("postgres", d.config.DatabaseURL)
	if err != nil {
		return err
	}

	if err := db.Ping(); err != nil {
		return err
	}

	d.db = db
	return nil
}

func (d *DB) Close() error {
	if d.db == nil {
		return nil
	}
	return d.db.Close()
}

func (d *DB) University() *UniversityRepository {
	if d.universityRepo != nil {
		return d.universityRepo
	}

	d.universityRepo = &UniversityRepository{database: d}
	return d.universityRepo
}

func (d *DB) Admin() *AdminRepository {
	if d.adminRepo != nil {
		return d.adminRepo
	}

	d.adminRepo = &AdminRepository{database: d}
	return d.adminRepo
}

func (d *DB) APIKey() *APIKeyRepository {
	if d.apiKeyRepo != nil {
		return d.apiKeyRepo
	}

	d.apiKeyRepo = &APIKeyRepository{database: d}
	return d.apiKeyRepo
}

func (d *DB) Diploma() *DiplomaRepository {
	if d.diplomaRepo != nil {
		return d.diplomaRepo
	}

	d.diplomaRepo = &DiplomaRepository{database: d, qrPayloadDecoder: d.qrPayloadDecoder}
	return d.diplomaRepo
}

func (d *DB) SigningKey() *SigningKeyRepository {
	if d.signingKeyRepo != nil {
		return d.signingKeyRepo
	}

	d.signingKeyRepo = &SigningKeyRepository{database: d}
	return d.signingKeyRepo
}

func (d *DB) SetQRPayloadDecoder(decoder QRPayloadDecoder) {
	d.qrPayloadDecoder = decoder
	if d.diplomaRepo != nil {
		d.diplomaRepo.qrPayloadDecoder = decoder
	}
}
