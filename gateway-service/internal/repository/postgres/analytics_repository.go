package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/diasoft/gateway-service/internal/model"
)

type AnalyticsRepository struct {
	database *DB
}

func (r *AnalyticsRepository) InsertVerificationEvent(ctx context.Context, event *model.VerificationEvent) error {
	_, err := r.database.db.ExecContext(
		ctx,
		`INSERT INTO verification_events (
			event_id,
			created_at,
			source_service,
			endpoint,
			request_id,
			vuz_id,
			vuz_code,
			diploma_hash,
			status,
			is_valid,
			country,
			city,
			client_ip_hash,
			user_agent
		) VALUES (
			$1, $2, $3, $4, $5,
			NULLIF($6, '')::uuid,
			NULLIF($7, ''),
			NULLIF($8, ''),
			$9, $10, NULLIF($11, ''), NULLIF($12, ''), NULLIF($13, ''), NULLIF($14, '')
		)
		ON CONFLICT (event_id) DO NOTHING`,
		event.EventID,
		event.CreatedAt,
		event.Source,
		event.Endpoint,
		nullIfEmpty(event.RequestID),
		event.VUZID,
		event.VUZCode,
		event.DiplomaHash,
		event.Status,
		event.Valid,
		event.Country,
		event.City,
		event.ClientIPHash,
		event.UserAgent,
	)
	return err
}

func (r *AnalyticsRepository) UniversityVerificationStats(ctx context.Context, vuzID string, filter model.VerificationStatsFilter) (*model.VerificationStatsResponse, error) {
	vuzCode, err := r.lookupVUZCode(ctx, vuzID)
	if err != nil {
		return nil, err
	}

	response := &model.VerificationStatsResponse{
		From:       filter.From,
		To:         filter.To,
		Statuses:   []model.VerificationStatusCount{},
		Timeseries: []model.VerificationTimeBucket{},
		Geography:  []model.VerificationGeoPoint{},
	}

	if err := r.database.db.QueryRowContext(
		ctx,
		`SELECT
			COUNT(*),
			COUNT(DISTINCT client_ip_hash)
		 FROM verification_events
		 WHERE created_at >= $1
		   AND created_at < $2
		   AND (vuz_id = $3::uuid OR (vuz_id IS NULL AND vuz_code = $4))`,
		filter.From,
		filter.To,
		vuzID,
		vuzCode,
	).Scan(&response.TotalChecks, &response.UniqueRequesters); err != nil {
		return nil, err
	}

	statuses, err := r.loadStatusDistribution(ctx, filter, "AND (vuz_id = $3::uuid OR (vuz_id IS NULL AND vuz_code = $4))", vuzID, vuzCode)
	if err != nil {
		return nil, err
	}
	response.Statuses = statuses

	timeseries, err := r.loadTimeseries(ctx, filter, "AND (vuz_id = $3::uuid OR (vuz_id IS NULL AND vuz_code = $4))", vuzID, vuzCode)
	if err != nil {
		return nil, err
	}
	response.Timeseries = timeseries

	geography, err := r.loadGeography(ctx, filter, "AND (vuz_id = $3::uuid OR (vuz_id IS NULL AND vuz_code = $4))", vuzID, vuzCode)
	if err != nil {
		return nil, err
	}
	response.Geography = geography

	return response, nil
}

func (r *AnalyticsRepository) AdminVerificationStats(ctx context.Context, filter model.VerificationStatsFilter) (*model.VerificationStatsResponse, error) {
	response := &model.VerificationStatsResponse{
		From:            filter.From,
		To:              filter.To,
		Statuses:        []model.VerificationStatusCount{},
		Timeseries:      []model.VerificationTimeBucket{},
		Geography:       []model.VerificationGeoPoint{},
		TopUniversities: []model.VerificationTopUniversity{},
	}

	if err := r.database.db.QueryRowContext(
		ctx,
		`SELECT
			COUNT(*),
			COUNT(DISTINCT client_ip_hash)
		 FROM verification_events
		 WHERE created_at >= $1
		   AND created_at < $2`,
		filter.From,
		filter.To,
	).Scan(&response.TotalChecks, &response.UniqueRequesters); err != nil {
		return nil, err
	}

	statuses, err := r.loadStatusDistribution(ctx, filter, "")
	if err != nil {
		return nil, err
	}
	response.Statuses = statuses

	timeseries, err := r.loadTimeseries(ctx, filter, "")
	if err != nil {
		return nil, err
	}
	response.Timeseries = timeseries

	geography, err := r.loadGeography(ctx, filter, "")
	if err != nil {
		return nil, err
	}
	response.Geography = geography

	topUniversities, err := r.loadTopUniversities(ctx, filter)
	if err != nil {
		return nil, err
	}
	response.TopUniversities = topUniversities

	return response, nil
}

func (r *AnalyticsRepository) loadStatusDistribution(ctx context.Context, filter model.VerificationStatsFilter, extraWhere string, args ...any) ([]model.VerificationStatusCount, error) {
	query := fmt.Sprintf(
		`SELECT status, COUNT(*)
		 FROM verification_events
		 WHERE created_at >= $1
		   AND created_at < $2
		   %s
		 GROUP BY status
		 ORDER BY COUNT(*) DESC, status ASC`,
		extraWhere,
	)

	rows, err := r.database.db.QueryContext(ctx, query, append([]any{filter.From, filter.To}, args...)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []model.VerificationStatusCount
	for rows.Next() {
		var item model.VerificationStatusCount
		if err := rows.Scan(&item.Status, &item.Count); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *AnalyticsRepository) loadTimeseries(ctx context.Context, filter model.VerificationStatsFilter, extraWhere string, args ...any) ([]model.VerificationTimeBucket, error) {
	query := fmt.Sprintf(
		`SELECT to_char(date_trunc('day', created_at), 'YYYY-MM-DD') AS day, COUNT(*)
		 FROM verification_events
		 WHERE created_at >= $1
		   AND created_at < $2
		   %s
		 GROUP BY day
		 ORDER BY day ASC`,
		extraWhere,
	)

	rows, err := r.database.db.QueryContext(ctx, query, append([]any{filter.From, filter.To}, args...)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []model.VerificationTimeBucket
	for rows.Next() {
		var item model.VerificationTimeBucket
		if err := rows.Scan(&item.Date, &item.Count); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *AnalyticsRepository) loadGeography(ctx context.Context, filter model.VerificationStatsFilter, extraWhere string, args ...any) ([]model.VerificationGeoPoint, error) {
	query := fmt.Sprintf(
		`SELECT COALESCE(country, ''), COALESCE(city, ''), COUNT(*)
		 FROM verification_events
		 WHERE created_at >= $1
		   AND created_at < $2
		   AND COALESCE(country, '') <> ''
		   %s
		 GROUP BY country, city
		 ORDER BY COUNT(*) DESC, country ASC, city ASC
		 LIMIT 20`,
		extraWhere,
	)

	rows, err := r.database.db.QueryContext(ctx, query, append([]any{filter.From, filter.To}, args...)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []model.VerificationGeoPoint
	for rows.Next() {
		var item model.VerificationGeoPoint
		if err := rows.Scan(&item.Country, &item.City, &item.Count); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *AnalyticsRepository) loadTopUniversities(ctx context.Context, filter model.VerificationStatsFilter) ([]model.VerificationTopUniversity, error) {
	rows, err := r.database.db.QueryContext(
		ctx,
		`SELECT
			COALESCE(u.id::text, ''),
			COALESCE(u.vuz_code, ve.vuz_code, ''),
			COALESCE(u.name, ''),
			COUNT(*)
		 FROM verification_events ve
		 LEFT JOIN universities u
		   ON u.id = ve.vuz_id
		    OR (ve.vuz_id IS NULL AND ve.vuz_code IS NOT NULL AND u.vuz_code = ve.vuz_code)
		 WHERE ve.created_at >= $1
		   AND ve.created_at < $2
		 GROUP BY COALESCE(u.id::text, ''), COALESCE(u.vuz_code, ve.vuz_code, ''), COALESCE(u.name, '')
		 ORDER BY COUNT(*) DESC, COALESCE(u.name, '') ASC
		 LIMIT 10`,
		filter.From,
		filter.To,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []model.VerificationTopUniversity
	for rows.Next() {
		var item model.VerificationTopUniversity
		if err := rows.Scan(&item.VUZID, &item.VUZCode, &item.Name, &item.Checks); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *AnalyticsRepository) lookupVUZCode(ctx context.Context, vuzID string) (string, error) {
	var vuzCode string
	if err := r.database.db.QueryRowContext(ctx, `SELECT vuz_code FROM universities WHERE id = $1`, vuzID).Scan(&vuzCode); err != nil {
		if err == sql.ErrNoRows {
			return "", err
		}
		return "", err
	}
	return vuzCode, nil
}

func nullIfEmpty(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func normalizeVerificationStatsFilter(filter model.VerificationStatsFilter) model.VerificationStatsFilter {
	from := filter.From.UTC()
	to := filter.To.UTC()
	if to.Before(from) {
		from, to = to, from
	}
	return model.VerificationStatsFilter{From: from, To: to}
}

func defaultVerificationStatsFilter() model.VerificationStatsFilter {
	to := time.Now().UTC()
	from := to.AddDate(0, 0, -30)
	return model.VerificationStatsFilter{From: from, To: to}
}
