package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/diasoft/gateway-service/internal/model"
)

type DiplomaRepository struct {
	database *DB
}

func (r *DiplomaRepository) CreateBatchWithRecords(ctx context.Context, vuzID string, records []model.DiplomaUploadRecord) (*model.Batch, error) {
	tx, err := r.database.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	batch := &model.Batch{}
	err = tx.QueryRowContext(
		ctx,
		`INSERT INTO batches (vuz_id, status, total_records, processed_records)
		 VALUES ($1, 'processing', $2, 0)
		 RETURNING id, vuz_id, status, total_records, processed_records, created_at, completed_at`,
		vuzID,
		len(records),
	).Scan(
		&batch.ID,
		&batch.VUZID,
		&batch.Status,
		&batch.TotalRecords,
		&batch.ProcessedRecords,
		&batch.CreatedAt,
		&batch.CompletedAt,
	)
	if err != nil {
		return nil, err
	}

	for index, record := range records {
		_, err := tx.ExecContext(
			ctx,
			`INSERT INTO batch_records
			 (batch_id, record_index, full_name, diploma_number, specialty, degree, faculty, year, raw_csv_row)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
			batch.ID,
			index,
			record.FullName,
			record.DiplomaNumber,
			record.Specialty,
			record.Degree,
			record.Faculty,
			record.Year,
			record.RawCSVRow,
		)
		if err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return batch, nil
}

func (r *DiplomaRepository) FailBatch(ctx context.Context, batchID string) error {
	_, err := r.database.db.ExecContext(
		ctx,
		`UPDATE batches
		 SET status = 'failed', completed_at = NOW()
		 WHERE id = $1`,
		batchID,
	)
	return err
}

func (r *DiplomaRepository) GetBatch(ctx context.Context, batchID, vuzID string) (*model.Batch, error) {
	batch := &model.Batch{}
	err := r.database.db.QueryRowContext(
		ctx,
		`SELECT
			b.id,
			b.vuz_id,
			b.status,
			b.total_records,
			b.processed_records,
			COALESCE((SELECT COUNT(*) FROM batch_records br WHERE br.batch_id = b.id AND br.status = 'error'), 0),
			b.created_at,
			b.completed_at
		 FROM batches b
		 WHERE b.id = $1 AND b.vuz_id = $2`,
		batchID,
		vuzID,
	).Scan(
		&batch.ID,
		&batch.VUZID,
		&batch.Status,
		&batch.TotalRecords,
		&batch.ProcessedRecords,
		&batch.FailedRecords,
		&batch.CreatedAt,
		&batch.CompletedAt,
	)
	if err != nil {
		return nil, err
	}

	return batch, nil
}

func (r *DiplomaRepository) GetBatchDownloadRows(ctx context.Context, batchID, vuzID string) ([]*model.BatchDownloadRow, error) {
	rows, err := r.database.db.QueryContext(
		ctx,
		`SELECT
			br.record_index,
			COALESCE(result.diploma_hash, ''),
			br.full_name,
			br.diploma_number,
			br.specialty,
			br.degree,
			br.faculty,
			br.year,
			COALESCE(result.qr_payload, ''),
			COALESCE(dh.status, br.status),
			br.error
		 FROM batch_records br
		 JOIN batches b ON b.id = br.batch_id
		 LEFT JOIN batch_results result
		   ON result.batch_id = br.batch_id
		  AND result.record_index = br.record_index
		 LEFT JOIN diploma_hashes dh
		   ON dh.hash = result.diploma_hash
		 WHERE br.batch_id = $1
		   AND b.vuz_id = $2
		 ORDER BY br.record_index`,
		batchID,
		vuzID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]*model.BatchDownloadRow, 0)
	for rows.Next() {
		item := &model.BatchDownloadRow{}
		var nullableError sql.NullString

		if err := rows.Scan(
			&item.RecordIndex,
			&item.DiplomaHash,
			&item.FullName,
			&item.DiplomaNumber,
			&item.Specialty,
			&item.Degree,
			&item.Faculty,
			&item.Year,
			&item.QRPayload,
			&item.Status,
			&nullableError,
		); err != nil {
			return nil, err
		}

		if nullableError.Valid {
			item.Error = &nullableError.String
		}

		result = append(result, item)
	}

	return result, rows.Err()
}

func (r *DiplomaRepository) MarkDiplomaRevoked(ctx context.Context, vuzID, hash string) error {
	result, err := r.database.db.ExecContext(
		ctx,
		`UPDATE diploma_hashes
		 SET status = 'revoked', revoked_at = NOW()
		 WHERE hash = $1 AND vuz_id = $2`,
		hash,
		vuzID,
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

func (r *DiplomaRepository) ApplyProcessingResult(ctx context.Context, result *model.KafkaProcessingResult) error {
	tx, err := r.database.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var (
		diplomaNumber string
		recordStatus  string
	)

	err = tx.QueryRowContext(
		ctx,
		`SELECT diploma_number, status
		 FROM batch_records
		 WHERE batch_id = $1 AND record_index = $2
		 FOR UPDATE`,
		result.BatchID,
		result.RecordIndex,
	).Scan(&diplomaNumber, &recordStatus)
	if err != nil {
		return err
	}

	if recordStatus != model.RecordStatusPending {
		return tx.Commit()
	}

	if result.Status != "ok" || result.EncryptedPayload == nil || result.QRPayload == nil || strings.TrimSpace(result.DiplomaHash) == "" {
		if err := r.markBatchRecordErrorTx(ctx, tx, result.BatchID, result.RecordIndex, stringOrDefault(result.Error, "processing error")); err != nil {
			return err
		}
		if err := r.incrementBatchProcessedTx(ctx, tx, result.BatchID); err != nil {
			return err
		}
		if err := r.finalizeBatchIfCompleteTx(ctx, tx, result.BatchID); err != nil {
			return err
		}
		return tx.Commit()
	}

	var existingHash string
	existingErr := tx.QueryRowContext(
		ctx,
		`SELECT hash
		 FROM diploma_hashes
		 WHERE vuz_id = $1 AND diploma_number = $2`,
		result.VUZID,
		diplomaNumber,
	).Scan(&existingHash)

	switch {
	case existingErr == sql.ErrNoRows:
		_, err = tx.ExecContext(
			ctx,
			`INSERT INTO diploma_hashes (hash, vuz_id, diploma_number, status, signature)
			 VALUES ($1, $2, $3, 'active', $4)`,
			result.DiplomaHash,
			result.VUZID,
			diplomaNumber,
			result.Signature,
		)
		if err != nil {
			return err
		}
	case existingErr == nil && existingHash != result.DiplomaHash:
		if err := r.markBatchRecordErrorTx(ctx, tx, result.BatchID, result.RecordIndex, "diploma number already exists"); err != nil {
			return err
		}
		if err := r.incrementBatchProcessedTx(ctx, tx, result.BatchID); err != nil {
			return err
		}
		if err := r.finalizeBatchIfCompleteTx(ctx, tx, result.BatchID); err != nil {
			return err
		}
		return tx.Commit()
	case existingErr != nil:
		return existingErr
	}

	_, err = tx.ExecContext(
		ctx,
		`INSERT INTO batch_results (batch_id, record_index, diploma_hash, encrypted_payload, qr_payload)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (batch_id, record_index)
		 DO UPDATE SET
			diploma_hash = EXCLUDED.diploma_hash,
			encrypted_payload = EXCLUDED.encrypted_payload,
			qr_payload = EXCLUDED.qr_payload`,
		result.BatchID,
		result.RecordIndex,
		result.DiplomaHash,
		*result.EncryptedPayload,
		*result.QRPayload,
	)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(
		ctx,
		`UPDATE batch_records
		 SET status = 'processed', error = NULL
		 WHERE batch_id = $1 AND record_index = $2`,
		result.BatchID,
		result.RecordIndex,
	)
	if err != nil {
		return err
	}

	if err := r.incrementBatchProcessedTx(ctx, tx, result.BatchID); err != nil {
		return err
	}
	if err := r.finalizeBatchIfCompleteTx(ctx, tx, result.BatchID); err != nil {
		return err
	}

	return tx.Commit()
}

func (r *DiplomaRepository) SearchStudents(ctx context.Context, diplomaNumber, fullName string) ([]*model.StudentSearchResult, error) {
	var (
		builder strings.Builder
		args    []interface{}
	)

	builder.WriteString(
		`SELECT
			dh.hash,
			br.diploma_number,
			br.full_name,
			br.specialty,
			br.degree,
			br.faculty,
			br.year,
			u.id,
			u.name,
			dh.status,
			dh.created_at
		 FROM diploma_hashes dh
		 JOIN batch_results result ON result.diploma_hash = dh.hash
		 JOIN batch_records br ON br.batch_id = result.batch_id AND br.record_index = result.record_index
		 JOIN universities u ON u.id = dh.vuz_id
		 WHERE 1 = 1`,
	)

	if diplomaNumber != "" {
		args = append(args, diplomaNumber)
		builder.WriteString(fmt.Sprintf(" AND br.diploma_number = $%d", len(args)))
	}

	if fullName != "" {
		args = append(args, "%"+strings.ToLower(fullName)+"%")
		builder.WriteString(fmt.Sprintf(" AND lower(br.full_name) LIKE $%d", len(args)))
	}

	builder.WriteString(" ORDER BY dh.created_at DESC LIMIT 50")

	rows, err := r.database.db.QueryContext(ctx, builder.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]*model.StudentSearchResult, 0)
	for rows.Next() {
		item := &model.StudentSearchResult{}
		if err := rows.Scan(
			&item.DiplomaHash,
			&item.DiplomaNumber,
			&item.FullName,
			&item.Specialty,
			&item.Degree,
			&item.Faculty,
			&item.Year,
			&item.UniversityID,
			&item.UniversityName,
			&item.Status,
			&item.CreatedAt,
		); err != nil {
			return nil, err
		}

		result = append(result, item)
	}

	return result, rows.Err()
}

func (r *DiplomaRepository) FindStudentByHash(ctx context.Context, diplomaHash string) (*model.StudentSearchResult, error) {
	item := &model.StudentSearchResult{}
	err := r.database.db.QueryRowContext(
		ctx,
		`SELECT
			dh.hash,
			br.diploma_number,
			br.full_name,
			br.specialty,
			br.degree,
			br.faculty,
			br.year,
			u.id,
			u.name,
			dh.status,
			dh.created_at
		 FROM diploma_hashes dh
		 JOIN batch_results result ON result.diploma_hash = dh.hash
		 JOIN batch_records br ON br.batch_id = result.batch_id AND br.record_index = result.record_index
		 JOIN universities u ON u.id = dh.vuz_id
		 WHERE dh.hash = $1
		 LIMIT 1`,
		diplomaHash,
	).Scan(
		&item.DiplomaHash,
		&item.DiplomaNumber,
		&item.FullName,
		&item.Specialty,
		&item.Degree,
		&item.Faculty,
		&item.Year,
		&item.UniversityID,
		&item.UniversityName,
		&item.Status,
		&item.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	return item, nil
}

func (r *DiplomaRepository) CreateShareLink(ctx context.Context, diplomaHash, token string, expiresAt time.Time) error {
	_, err := r.database.db.ExecContext(
		ctx,
		`INSERT INTO share_links (diploma_hash, token, expires_at)
		 VALUES ($1, $2, $3)`,
		diplomaHash,
		token,
		expiresAt,
	)
	return err
}

func (r *DiplomaRepository) FindShareLink(ctx context.Context, token string) (*model.ShareLink, error) {
	item := &model.ShareLink{}
	err := r.database.db.QueryRowContext(
		ctx,
		`SELECT id, diploma_hash, token, expires_at, used_count, created_at
		 FROM share_links
		 WHERE token = $1`,
		token,
	).Scan(
		&item.ID,
		&item.DiplomaHash,
		&item.Token,
		&item.ExpiresAt,
		&item.UsedCount,
		&item.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	return item, nil
}

func (r *DiplomaRepository) IncrementShareLinkUsage(ctx context.Context, token string) error {
	_, err := r.database.db.ExecContext(
		ctx,
		`UPDATE share_links
		 SET used_count = used_count + 1
		 WHERE token = $1`,
		token,
	)
	return err
}

func (r *DiplomaRepository) GetVerificationSnapshot(ctx context.Context, diplomaHash string) (*model.VerificationSnapshot, error) {
	item := &model.VerificationSnapshot{}
	var publicKey sql.NullString
	var signature sql.NullString

	err := r.database.db.QueryRowContext(
		ctx,
		`SELECT
			dh.hash,
			dh.diploma_number,
			br.full_name,
			br.specialty,
			br.degree,
			br.faculty,
			br.year,
			u.id,
			u.name,
			u.public_key,
			dh.status,
			dh.signature,
			dh.created_at,
			result.qr_payload
		 FROM diploma_hashes dh
		 JOIN universities u ON u.id = dh.vuz_id
		 JOIN batch_results result ON result.diploma_hash = dh.hash
		 JOIN batch_records br ON br.batch_id = result.batch_id AND br.record_index = result.record_index
		 WHERE dh.hash = $1
		 LIMIT 1`,
		diplomaHash,
	).Scan(
		&item.DiplomaHash,
		&item.DiplomaNumber,
		&item.FullName,
		&item.Specialty,
		&item.Degree,
		&item.Faculty,
		&item.Year,
		&item.UniversityID,
		&item.UniversityName,
		&publicKey,
		&item.Status,
		&signature,
		&item.CreatedAt,
		&item.BatchResultJWT,
	)
	if err != nil {
		return nil, err
	}

	if publicKey.Valid {
		item.PublicKey = &publicKey.String
	}
	if signature.Valid {
		item.Signature = &signature.String
	}

	return item, nil
}

func (r *DiplomaRepository) markBatchRecordErrorTx(ctx context.Context, tx *sql.Tx, batchID string, recordIndex int, message string) error {
	_, err := tx.ExecContext(
		ctx,
		`UPDATE batch_records
		 SET status = 'error', error = $3
		 WHERE batch_id = $1 AND record_index = $2`,
		batchID,
		recordIndex,
		message,
	)
	return err
}

func (r *DiplomaRepository) incrementBatchProcessedTx(ctx context.Context, tx *sql.Tx, batchID string) error {
	_, err := tx.ExecContext(
		ctx,
		`UPDATE batches
		 SET processed_records = processed_records + 1
		 WHERE id = $1`,
		batchID,
	)
	return err
}

func (r *DiplomaRepository) finalizeBatchIfCompleteTx(ctx context.Context, tx *sql.Tx, batchID string) error {
	var (
		total     int
		processed int
		errorsCnt int
	)

	err := tx.QueryRowContext(
		ctx,
		`SELECT
			total_records,
			processed_records,
			(SELECT COUNT(*) FROM batch_records WHERE batch_id = $1 AND status = 'error')
		 FROM batches
		 WHERE id = $1`,
		batchID,
	).Scan(&total, &processed, &errorsCnt)
	if err != nil {
		return err
	}

	if processed < total {
		return nil
	}

	status := model.BatchStatusCompleted
	if errorsCnt > 0 {
		status = model.BatchStatusFailed
	}

	_, err = tx.ExecContext(
		ctx,
		`UPDATE batches
		 SET status = $2, completed_at = COALESCE(completed_at, NOW())
		 WHERE id = $1`,
		batchID,
		status,
	)
	return err
}

func stringOrDefault(value *string, defaultValue string) string {
	if value == nil {
		return defaultValue
	}

	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return defaultValue
	}

	return trimmed
}
