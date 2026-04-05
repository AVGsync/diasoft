package service

import (
	"context"
	"time"

	"github.com/diasoft/gateway-service/internal/model"
)

type AnalyticsRepository interface {
	InsertVerificationEvent(ctx context.Context, event *model.VerificationEvent) error
	UniversityVerificationStats(ctx context.Context, vuzID string, filter model.VerificationStatsFilter) (*model.VerificationStatsResponse, error)
	AdminVerificationStats(ctx context.Context, filter model.VerificationStatsFilter) (*model.VerificationStatsResponse, error)
}

type AnalyticsService struct {
	repo AnalyticsRepository
}

func NewAnalyticsService(repo AnalyticsRepository) *AnalyticsService {
	return &AnalyticsService{repo: repo}
}

func (s *AnalyticsService) HandleVerificationEvent(ctx context.Context, event *model.VerificationEvent) error {
	return s.repo.InsertVerificationEvent(ctx, event)
}

func (s *AnalyticsService) UniversityVerificationStats(ctx context.Context, vuzID string, from, to time.Time) (*model.VerificationStatsResponse, error) {
	filter := normalizeStatsWindow(from, to)
	return s.repo.UniversityVerificationStats(ctx, vuzID, filter)
}

func (s *AnalyticsService) AdminVerificationStats(ctx context.Context, from, to time.Time) (*model.VerificationStatsResponse, error) {
	filter := normalizeStatsWindow(from, to)
	return s.repo.AdminVerificationStats(ctx, filter)
}

func normalizeStatsWindow(from, to time.Time) model.VerificationStatsFilter {
	if from.IsZero() || to.IsZero() {
		to = time.Now().UTC()
		from = to.AddDate(0, 0, -30)
	}

	from = from.UTC()
	to = to.UTC()
	if !to.After(from) {
		to = from.Add(24 * time.Hour)
	}

	return model.VerificationStatsFilter{
		From: from,
		To:   to,
	}
}
