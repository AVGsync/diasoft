package postgres

import (
	"context"
	"database/sql"
	"errors"

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
		`INSERT INTO universities (name, vuz_code, inn, ogrn, email, password_hash)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, name, vuz_code, inn, ogrn, email, password_hash, public_key, status, created_at`,
		request.Name,
		request.VuzCode,
		request.INN,
		request.OGRN,
		request.Email,
		passwordHash,
	).Scan(
		&university.ID,
		&university.Name,
		&university.VuzCode,
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

func (r *UniversityRepository) UpsertDemo(ctx context.Context, request *model.RegisterUniversityRequest, passwordHash, status string) (*model.University, error) {
	tx, err := r.database.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	university := &model.University{}
	var publicKey sql.NullString
	var existingID string

	err = tx.QueryRowContext(
		ctx,
		`SELECT id
		 FROM universities
		 WHERE email = $1 OR vuz_code = $2
		 ORDER BY CASE WHEN email = $1 THEN 0 ELSE 1 END
		 LIMIT 1
		 FOR UPDATE`,
		request.Email,
		request.VuzCode,
	).Scan(&existingID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	if errors.Is(err, sql.ErrNoRows) {
		err = tx.QueryRowContext(
			ctx,
			`INSERT INTO universities (name, vuz_code, inn, ogrn, email, password_hash, status)
			 VALUES ($1, $2, $3, $4, $5, $6, $7)
			 RETURNING id, name, vuz_code, inn, ogrn, email, password_hash, public_key, status, created_at`,
			request.Name,
			request.VuzCode,
			request.INN,
			request.OGRN,
			request.Email,
			passwordHash,
			status,
		).Scan(
			&university.ID,
			&university.Name,
			&university.VuzCode,
			&university.INN,
			&university.OGRN,
			&university.Email,
			&university.PasswordHash,
			&publicKey,
			&university.Status,
			&university.CreatedAt,
		)
	} else {
		err = tx.QueryRowContext(
			ctx,
			`UPDATE universities
			 SET name = $2,
			     vuz_code = $3,
			     inn = $4,
			     ogrn = $5,
			     email = $6,
			     password_hash = $7,
			     status = $8
			 WHERE id = $1
			 RETURNING id, name, vuz_code, inn, ogrn, email, password_hash, public_key, status, created_at`,
			existingID,
			request.Name,
			request.VuzCode,
			request.INN,
			request.OGRN,
			request.Email,
			passwordHash,
			status,
		).Scan(
			&university.ID,
			&university.Name,
			&university.VuzCode,
			&university.INN,
			&university.OGRN,
			&university.Email,
			&university.PasswordHash,
			&publicKey,
			&university.Status,
			&university.CreatedAt,
		)
	}
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
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
		`SELECT id, name, vuz_code, inn, ogrn, email, password_hash, public_key, status, created_at
		 FROM universities
		 WHERE email = $1`,
		email,
	).Scan(
		&university.ID,
		&university.Name,
		&university.VuzCode,
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
		`SELECT id, name, vuz_code, inn, ogrn, email, password_hash, public_key, status, created_at
		 FROM universities
		 WHERE id = $1`,
		id,
	).Scan(
		&university.ID,
		&university.Name,
		&university.VuzCode,
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

func (r *UniversityRepository) List(ctx context.Context) ([]*model.University, error) {
	rows, err := r.database.db.QueryContext(
		ctx,
		`SELECT id, name, vuz_code, inn, ogrn, email, password_hash, public_key, status, created_at
		 FROM universities
		 ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]*model.University, 0)
	for rows.Next() {
		item := &model.University{}
		var publicKey sql.NullString

		if err := rows.Scan(
			&item.ID,
			&item.Name,
			&item.VuzCode,
			&item.INN,
			&item.OGRN,
			&item.Email,
			&item.PasswordHash,
			&publicKey,
			&item.Status,
			&item.CreatedAt,
		); err != nil {
			return nil, err
		}

		if publicKey.Valid {
			item.PublicKey = &publicKey.String
		}

		result = append(result, item)
	}

	return result, rows.Err()
}

func (r *UniversityRepository) Activate(ctx context.Context, id string) (*model.University, error) {
	return r.UpdateStatus(ctx, id, model.UniversityStatusActive)
}

func (r *UniversityRepository) UpdateStatus(ctx context.Context, id, status string) (*model.University, error) {
	university := &model.University{}
	var publicKey sql.NullString

	err := r.database.db.QueryRowContext(
		ctx,
		`UPDATE universities
		 SET status = $2
		 WHERE id = $1
		 RETURNING id, name, vuz_code, inn, ogrn, email, password_hash, public_key, status, created_at`,
		id,
		status,
	).Scan(
		&university.ID,
		&university.Name,
		&university.VuzCode,
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

func (r *UniversityRepository) DeleteCascade(ctx context.Context, id string) error {
	tx, err := r.database.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var exists string
	if err := tx.QueryRowContext(ctx, `SELECT id FROM universities WHERE id = $1`, id).Scan(&exists); err != nil {
		return err
	}

	if _, err := tx.ExecContext(
		ctx,
		`DELETE FROM share_links
		 WHERE diploma_hash IN (
		 	SELECT hash
		 	FROM diploma_hashes
		 	WHERE vuz_id = $1
		 )`,
		id,
	); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM batches WHERE vuz_id = $1`, id); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM diploma_hashes WHERE vuz_id = $1`, id); err != nil {
		return err
	}

	result, err := tx.ExecContext(ctx, `DELETE FROM universities WHERE id = $1`, id)
	if err != nil {
		return err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return sql.ErrNoRows
	}

	return tx.Commit()
}

func (r *UniversityRepository) UpdatePublicKey(ctx context.Context, id, publicKey string) error {
	result, err := r.database.db.ExecContext(
		ctx,
		`UPDATE universities
		 SET public_key = $2
		 WHERE id = $1`,
		id,
		publicKey,
	)
	if err != nil {
		return err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return sql.ErrNoRows
	}

	return nil
}
