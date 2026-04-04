package postgres

import (
	"context"
	"database/sql"
	"log/slog"
	"strings"
	"time"

	"github.com/diasoft/gateway-service/internal/model"
)

type DiplomaRepository struct {
	database            *DB
	qrPayloadDecoder    QRPayloadDecoder
	recordPayloadCipher RecordPayloadCipher
}

func (r *DiplomaRepository) CreateBatchWithRecords(ctx context.Context, vuzID string, records []model.DiplomaUploadRecord) (*model.Batch, error) {
	batch := &model.Batch{}

	tx, err := r.database.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

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
		encryptedPayload, err := encodeBatchRecordPayload(r.recordPayloadCipher, record)
		if err != nil {
			return nil, err
		}

		_, err = tx.ExecContext(
			ctx,
			`INSERT INTO batch_record_payloads (batch_id, record_index, encrypted_payload)
			 VALUES ($1, $2, $3)`,
			batch.ID,
			index,
			encryptedPayload,
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
			COALESCE((SELECT COUNT(*) FROM batch_results br WHERE br.batch_id = b.id AND br.status = 'error'), 0),
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

func (r *DiplomaRepository) ListBatches(ctx context.Context, vuzID string, limit int) ([]*model.Batch, error) {
	if limit <= 0 {
		limit = 20
	}

	rows, err := r.database.db.QueryContext(
		ctx,
		`SELECT
			b.id,
			b.vuz_id,
			b.status,
			b.total_records,
			b.processed_records,
			COALESCE((SELECT COUNT(*) FROM batch_results br WHERE br.batch_id = b.id AND br.status = 'error'), 0),
			b.created_at,
			b.completed_at
		 FROM batches b
		 WHERE b.vuz_id = $1
		 ORDER BY b.created_at DESC
		 LIMIT $2`,
		vuzID,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]*model.Batch, 0)
	for rows.Next() {
		item := &model.Batch{}
		if err := rows.Scan(
			&item.ID,
			&item.VUZID,
			&item.Status,
			&item.TotalRecords,
			&item.ProcessedRecords,
			&item.FailedRecords,
			&item.CreatedAt,
			&item.CompletedAt,
		); err != nil {
			return nil, err
		}

		result = append(result, item)
	}

	return result, rows.Err()
}

func (r *DiplomaRepository) GetBatchDownloadRows(ctx context.Context, batchID, vuzID string) ([]*model.BatchDownloadRow, error) {
	rows, err := r.database.db.QueryContext(
		ctx,
		`SELECT
			result.record_index,
			result.diploma_hash,
			result.qr_payload,
			result.status,
			result.error,
			dh.status,
			payloads.encrypted_payload
		 FROM batch_results result
		 JOIN batches b ON b.id = result.batch_id
		 LEFT JOIN diploma_hashes dh ON dh.hash = result.diploma_hash
		 LEFT JOIN batch_record_payloads payloads ON payloads.batch_id = result.batch_id AND payloads.record_index = result.record_index
		 WHERE result.batch_id = $1
		   AND b.vuz_id = $2
		 ORDER BY result.record_index`,
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
		var (
			diplomaHash      sql.NullString
			qrPayload        sql.NullString
			rowStatus        string
			rowError         sql.NullString
			diplomaStatus    sql.NullString
			encryptedPayload sql.NullString
		)

		if err := rows.Scan(
			&item.RecordIndex,
			&diplomaHash,
			&qrPayload,
			&rowStatus,
			&rowError,
			&diplomaStatus,
			&encryptedPayload,
		); err != nil {
			return nil, err
		}

		if diplomaHash.Valid {
			item.DiplomaHash = diplomaHash.String
		}
		if rowError.Valid {
			item.Error = &rowError.String
		}

		item.Status = rowStatus
		if rowStatus != model.RecordStatusError && diplomaStatus.Valid {
			item.Status = diplomaStatus.String
		}

		if encryptedPayload.Valid {
			payload, err := decodeBatchRecordPayload(r.recordPayloadCipher, encryptedPayload.String)
			if err != nil {
				return nil, err
			}
			if payload != nil {
				item.FullName = payload.FullName
				item.DiplomaNumber = payload.DiplomaNumber
				item.Specialty = payload.Specialty
				item.Degree = payload.Degree
				item.Faculty = payload.Faculty
				item.Year = payload.Year
			}
		}

		if qrPayload.Valid && strings.TrimSpace(qrPayload.String) != "" {
			item.QRPayload = qrPayload.String

			if item.FullName == "" || item.DiplomaNumber == "" || item.Specialty == "" || item.Degree == "" || item.Faculty == "" || item.Year == 0 {
				payload, err := parseQRPayload(r.qrPayloadDecoder, qrPayload.String)
				if err != nil {
					return nil, err
				}

				item.FullName = firstNonEmpty(item.FullName, payload.FullName)
				item.DiplomaNumber = firstNonEmpty(item.DiplomaNumber, payload.DiplomaNumber)
				item.Specialty = firstNonEmpty(item.Specialty, payload.Specialty)
				item.Degree = firstNonEmpty(item.Degree, payload.Degree)
				item.Faculty = firstNonEmpty(item.Faculty, payload.Faculty)
				item.Year = firstNonZero(item.Year, payload.Year)
			}
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
	slog.Info(
		"starting processing result transaction",
		"batch_id", result.BatchID,
		"record_index", result.RecordIndex,
		"vuz_id", result.VUZID,
		"status", result.Status,
	)

	tx, err := r.database.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var existingStatus string
	err = tx.QueryRowContext(
		ctx,
		`SELECT status
		 FROM batch_results
		 WHERE batch_id = $1 AND record_index = $2
		 FOR UPDATE`,
		result.BatchID,
		result.RecordIndex,
	).Scan(&existingStatus)
	switch {
	case err == nil:
		slog.Warn(
			"skipping kafka result because batch result already exists",
			"batch_id", result.BatchID,
			"record_index", result.RecordIndex,
			"existing_status", existingStatus,
		)
		return tx.Commit()
	case err != sql.ErrNoRows:
		return err
	}

	if result.Status != "ok" || result.QRPayload == nil || strings.TrimSpace(result.DiplomaHash) == "" {
		slog.Warn(
			"storing batch result as error because kafka result is incomplete",
			"batch_id", result.BatchID,
			"record_index", result.RecordIndex,
			"status", result.Status,
			"has_qr_payload", result.QRPayload != nil,
			"has_diploma_hash", strings.TrimSpace(result.DiplomaHash) != "",
			"error", stringOrDefault(result.Error, "processing error"),
		)
		if err := r.insertBatchResultErrorTx(ctx, tx, result.BatchID, result.RecordIndex, stringOrDefault(result.Error, "processing error")); err != nil {
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

	qrData, err := parseQRPayload(r.qrPayloadDecoder, *result.QRPayload)
	if err != nil || strings.TrimSpace(qrData.DiplomaNumber) == "" {
		slog.Warn(
			"storing batch result as error because qr payload is invalid",
			"batch_id", result.BatchID,
			"record_index", result.RecordIndex,
			"parse_error", err,
		)
		if err := r.insertBatchResultErrorTx(ctx, tx, result.BatchID, result.RecordIndex, "qr payload does not contain diploma number"); err != nil {
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

	if strings.TrimSpace(qrData.VUZID) != "" && qrData.VUZID != result.VUZID {
		slog.Warn(
			"storing batch result as error because qr payload university does not match",
			"batch_id", result.BatchID,
			"record_index", result.RecordIndex,
			"result_vuz_id", result.VUZID,
			"qr_vuz_id", qrData.VUZID,
		)
		if err := r.insertBatchResultErrorTx(ctx, tx, result.BatchID, result.RecordIndex, "qr payload university mismatch"); err != nil {
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
		qrData.DiplomaNumber,
	).Scan(&existingHash)

	switch {
	case existingErr == sql.ErrNoRows:
		_, err = tx.ExecContext(
			ctx,
			`INSERT INTO diploma_hashes (hash, vuz_id, diploma_number, status)
			 VALUES ($1, $2, $3, 'active')`,
			result.DiplomaHash,
			result.VUZID,
			qrData.DiplomaNumber,
		)
		if err != nil {
			return err
		}
		slog.Info(
			"inserted diploma hash",
			"batch_id", result.BatchID,
			"record_index", result.RecordIndex,
			"diploma_hash", result.DiplomaHash,
			"diploma_number", qrData.DiplomaNumber,
		)
	case existingErr == nil && existingHash != result.DiplomaHash:
		slog.Warn(
			"storing batch result as error because diploma number already exists with another hash",
			"batch_id", result.BatchID,
			"record_index", result.RecordIndex,
			"diploma_number", qrData.DiplomaNumber,
			"existing_hash", existingHash,
			"incoming_hash", result.DiplomaHash,
		)
		if err := r.insertBatchResultErrorTx(ctx, tx, result.BatchID, result.RecordIndex, "diploma number already exists"); err != nil {
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
		`INSERT INTO batch_results (batch_id, record_index, diploma_hash, qr_payload, status, error)
		 VALUES ($1, $2, $3, $4, 'ok', NULL)`,
		result.BatchID,
		result.RecordIndex,
		result.DiplomaHash,
		*result.QRPayload,
	)
	if err != nil {
		return err
	}
	slog.Info(
		"inserted batch result",
		"batch_id", result.BatchID,
		"record_index", result.RecordIndex,
		"diploma_hash", result.DiplomaHash,
	)

	if err := r.incrementBatchProcessedTx(ctx, tx, result.BatchID); err != nil {
		return err
	}
	if err := r.finalizeBatchIfCompleteTx(ctx, tx, result.BatchID); err != nil {
		return err
	}

	slog.Info(
		"finished processing result transaction",
		"batch_id", result.BatchID,
		"record_index", result.RecordIndex,
		"diploma_hash", result.DiplomaHash,
	)

	return tx.Commit()
}

func (r *DiplomaRepository) SearchStudents(ctx context.Context, diplomaNumber, fullName string) ([]*model.StudentSearchResult, error) {
	query := `
		SELECT
			dh.hash,
			result.qr_payload,
			u.id,
			u.name,
			dh.status,
			dh.created_at
		FROM diploma_hashes dh
		JOIN LATERAL (
			SELECT batch_id, record_index, qr_payload
			FROM batch_results
			WHERE diploma_hash = dh.hash
			  AND status = 'ok'
			ORDER BY created_at DESC
			LIMIT 1
		) result ON TRUE
		JOIN universities u ON u.id = dh.vuz_id
		WHERE 1 = 1`

	args := make([]interface{}, 0, 1)
	if strings.TrimSpace(diplomaNumber) != "" {
		args = append(args, diplomaNumber)
		query += ` AND dh.diploma_number = $1`
	}
	query += ` ORDER BY dh.created_at DESC`

	rows, err := r.database.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	normalizedName := strings.ToLower(strings.TrimSpace(fullName))
	result := make([]*model.StudentSearchResult, 0)

	for rows.Next() {
		item := &model.StudentSearchResult{}
		var (
			qrPayload string
		)

		if err := rows.Scan(
			&item.DiplomaHash,
			&qrPayload,
			&item.UniversityID,
			&item.UniversityName,
			&item.Status,
			&item.CreatedAt,
		); err != nil {
			return nil, err
		}

		payload, err := parseQRPayload(r.qrPayloadDecoder, qrPayload)
		if err != nil {
			return nil, err
		}

		if normalizedName != "" && !strings.Contains(strings.ToLower(payload.FullName), normalizedName) {
			continue
		}

		item.DiplomaNumber = payload.DiplomaNumber
		item.FullName = payload.FullName
		item.Specialty = payload.Specialty
		item.Degree = payload.Degree
		item.Faculty = payload.Faculty
		item.Year = payload.Year

		result = append(result, item)
		if len(result) >= 50 {
			break
		}
	}

	return result, rows.Err()
}

func (r *DiplomaRepository) FindStudentByHash(ctx context.Context, diplomaHash string) (*model.StudentSearchResult, error) {
	item := &model.StudentSearchResult{}
	var (
		qrPayload string
	)

	err := r.database.db.QueryRowContext(
		ctx,
		`SELECT
			dh.hash,
			result.qr_payload,
			u.id,
			u.name,
			dh.status,
			dh.created_at
		 FROM diploma_hashes dh
		 JOIN LATERAL (
		 	SELECT batch_id, record_index, qr_payload
		 	FROM batch_results
		 	WHERE diploma_hash = dh.hash
		 	  AND status = 'ok'
		 	ORDER BY created_at DESC
		 	LIMIT 1
		 ) result ON TRUE
		 JOIN universities u ON u.id = dh.vuz_id
		 WHERE dh.hash = $1
		 LIMIT 1`,
		diplomaHash,
	).Scan(
		&item.DiplomaHash,
		&qrPayload,
		&item.UniversityID,
		&item.UniversityName,
		&item.Status,
		&item.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	payload, err := parseQRPayload(r.qrPayloadDecoder, qrPayload)
	if err != nil {
		return nil, err
	}

	item.DiplomaNumber = payload.DiplomaNumber
	item.FullName = payload.FullName
	item.Specialty = payload.Specialty
	item.Degree = payload.Degree
	item.Faculty = payload.Faculty
	item.Year = payload.Year

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

func (r *DiplomaRepository) insertBatchResultErrorTx(ctx context.Context, tx *sql.Tx, batchID string, recordIndex int, message string) error {
	_, err := tx.ExecContext(
		ctx,
		`INSERT INTO batch_results (batch_id, record_index, diploma_hash, qr_payload, status, error)
		 VALUES ($1, $2, NULL, NULL, 'error', $3)`,
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
			(SELECT COUNT(*) FROM batch_results WHERE batch_id = $1 AND status = 'error')
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

func firstNonZero(values ...int) int {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}
