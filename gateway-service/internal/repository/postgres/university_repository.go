package postgres

import (
	"context"
	"database/sql"

	"github.com/diasoft/gateway-service/internal/model"
)

type UniversityRepository struct {
	database *DB
}

func (r *UniversityRepository) Create(ctx context.Context, request *model.RegisterUniversityRequest, passwordHash string) (*model.University, error) {
	university := &model.University{}
	var publicKey sql.NullString

	err := r.database.db.QueryRowContext(
		ctx,
		`INSERT INTO universities (name, inn, ogrn, email, password_hash, public_key)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, name, inn, ogrn, email, password_hash, public_key, status, created_at`,
		request.Name,
		request.INN,
		request.OGRN,
		request.Email,
		passwordHash,
		request.PublicKey,
	).Scan(
		&university.ID,
		&university.Name,
		&university.INN,
		&university.OGRN,
		&university.Email,
		&university.PasswordHash,
		&publicKey,
		&university.Status,
		&university.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	if publicKey.Valid {
		university.PublicKey = &publicKey.String
	}

	return university, nil
}

func (r *UniversityRepository) FindByEmail(ctx context.Context, email string) (*model.University, error) {
	university := &model.University{}
	var publicKey sql.NullString

	err := r.database.db.QueryRowContext(
		ctx,
		`SELECT id, name, inn, ogrn, email, password_hash, public_key, status, created_at
		 FROM universities
		 WHERE email = $1`,
		email,
	).Scan(
		&university.ID,
		&university.Name,
		&university.INN,
		&university.OGRN,
		&university.Email,
		&university.PasswordHash,
		&publicKey,
		&university.Status,
		&university.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	if publicKey.Valid {
		university.PublicKey = &publicKey.String
	}

	return university, nil
}

func (r *UniversityRepository) FindByID(ctx context.Context, id string) (*model.University, error) {
	university := &model.University{}
	var publicKey sql.NullString

	err := r.database.db.QueryRowContext(
		ctx,
		`SELECT id, name, inn, ogrn, email, password_hash, public_key, status, created_at
		 FROM universities
		 WHERE id = $1`,
		id,
	).Scan(
		&university.ID,
		&university.Name,
		&university.INN,
		&university.OGRN,
		&university.Email,
		&university.PasswordHash,
		&publicKey,
		&university.Status,
		&university.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	if publicKey.Valid {
		university.PublicKey = &publicKey.String
	}

	return university, nil
}

func (r *UniversityRepository) Activate(ctx context.Context, id string) (*model.University, error) {
	university := &model.University{}
	var publicKey sql.NullString

	err := r.database.db.QueryRowContext(
		ctx,
		`UPDATE universities
		 SET status = 'active'
		 WHERE id = $1
		 RETURNING id, name, inn, ogrn, email, password_hash, public_key, status, created_at`,
		id,
	).Scan(
		&university.ID,
		&university.Name,
		&university.INN,
		&university.OGRN,
		&university.Email,
		&university.PasswordHash,
		&publicKey,
		&university.Status,
		&university.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	if publicKey.Valid {
		university.PublicKey = &publicKey.String
	}

	return university, nil
}
