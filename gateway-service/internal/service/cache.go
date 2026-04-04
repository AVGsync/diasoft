package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/diasoft/gateway-service/internal/infrastructure/rediscache"
	"github.com/diasoft/gateway-service/internal/model"
)

type GatewayCacheConfig struct {
	AdminStatsTTL        time.Duration
	UniversitiesListTTL  time.Duration
	UniversityProfileTTL time.Duration
	BatchStatusTTL       time.Duration
}

type adminServiceContract interface {
	ApproveUniversity(ctx context.Context, id string) (*model.University, error)
	GetUniversity(ctx context.Context, id string) (*model.University, error)
	ListUniversities(ctx context.Context) ([]*model.University, error)
	UpdateUniversityStatus(ctx context.Context, id, status string) (*model.University, error)
	DeleteUniversity(ctx context.Context, id string) error
	Stats(ctx context.Context) (*model.AdminStats, error)
}

type universityCabinetContract interface {
	Profile(ctx context.Context, vuzID string) (*model.University, error)
	ListBatches(ctx context.Context, vuzID string, limit int) ([]*model.Batch, error)
}

type diplomaServiceContract interface {
	Upload(ctx context.Context, vuzID string, records []model.DiplomaUploadRecord) (*model.BatchUploadResponse, error)
	GetBatch(ctx context.Context, batchID, vuzID string) (*model.Batch, error)
	DownloadBatch(ctx context.Context, batchID, vuzID string) ([]byte, error)
	Revoke(ctx context.Context, vuzID, hash string) error
	HandleProcessingResult(ctx context.Context, result *model.KafkaProcessingResult) error
}

type CachedAdminService struct {
	next   adminServiceContract
	cache  *rediscache.Client
	logger *slog.Logger
	cfg    GatewayCacheConfig
}

type CachedUniversityCabinetService struct {
	next   universityCabinetContract
	cache  *rediscache.Client
	logger *slog.Logger
	cfg    GatewayCacheConfig
}

type CachedDiplomaService struct {
	next   diplomaServiceContract
	cache  *rediscache.Client
	logger *slog.Logger
	cfg    GatewayCacheConfig
}

func NewCachedAdminService(next adminServiceContract, cache *rediscache.Client, logger *slog.Logger, cfg GatewayCacheConfig) *CachedAdminService {
	if logger == nil {
		logger = slog.Default()
	}
	return &CachedAdminService{next: next, cache: cache, logger: logger, cfg: cfg}
}

func NewCachedUniversityCabinetService(next universityCabinetContract, cache *rediscache.Client, logger *slog.Logger, cfg GatewayCacheConfig) *CachedUniversityCabinetService {
	if logger == nil {
		logger = slog.Default()
	}
	return &CachedUniversityCabinetService{next: next, cache: cache, logger: logger, cfg: cfg}
}

func NewCachedDiplomaService(next diplomaServiceContract, cache *rediscache.Client, logger *slog.Logger, cfg GatewayCacheConfig) *CachedDiplomaService {
	if logger == nil {
		logger = slog.Default()
	}
	return &CachedDiplomaService{next: next, cache: cache, logger: logger, cfg: cfg}
}

func (s *CachedAdminService) ApproveUniversity(ctx context.Context, id string) (*model.University, error) {
	university, err := s.next.ApproveUniversity(ctx, id)
	if err != nil {
		return nil, err
	}
	s.invalidateUniversityCaches(ctx, id)
	return university, nil
}

func (s *CachedAdminService) GetUniversity(ctx context.Context, id string) (*model.University, error) {
	key := adminUniversityCacheKey(id)

	var cached model.University
	found, err := s.cache.GetJSON(ctx, key, &cached)
	if err == nil && found {
		return &cached, nil
	}
	if err != nil {
		s.logger.Warn("gateway cache read failed", "cache_key", key, "error", err)
	}

	university, err := s.next.GetUniversity(ctx, id)
	if err != nil {
		return nil, err
	}
	s.storeCache(ctx, key, university, s.cfg.UniversityProfileTTL)
	return university, nil
}

func (s *CachedAdminService) ListUniversities(ctx context.Context) ([]*model.University, error) {
	key := adminUniversitiesCacheKey()

	var cached []*model.University
	found, err := s.cache.GetJSON(ctx, key, &cached)
	if err == nil && found {
		return cached, nil
	}
	if err != nil {
		s.logger.Warn("gateway cache read failed", "cache_key", key, "error", err)
	}

	universities, err := s.next.ListUniversities(ctx)
	if err != nil {
		return nil, err
	}
	s.storeCache(ctx, key, universities, s.cfg.UniversitiesListTTL)
	return universities, nil
}

func (s *CachedAdminService) UpdateUniversityStatus(ctx context.Context, id, status string) (*model.University, error) {
	university, err := s.next.UpdateUniversityStatus(ctx, id, status)
	if err != nil {
		return nil, err
	}
	s.invalidateUniversityCaches(ctx, id)
	return university, nil
}

func (s *CachedAdminService) DeleteUniversity(ctx context.Context, id string) error {
	if err := s.next.DeleteUniversity(ctx, id); err != nil {
		return err
	}
	s.invalidateUniversityCaches(ctx, id)
	return nil
}

func (s *CachedAdminService) Stats(ctx context.Context) (*model.AdminStats, error) {
	key := adminStatsCacheKey()

	var cached model.AdminStats
	found, err := s.cache.GetJSON(ctx, key, &cached)
	if err == nil && found {
		return &cached, nil
	}
	if err != nil {
		s.logger.Warn("gateway cache read failed", "cache_key", key, "error", err)
	}

	stats, err := s.next.Stats(ctx)
	if err != nil {
		return nil, err
	}
	s.storeCache(ctx, key, stats, s.cfg.AdminStatsTTL)
	return stats, nil
}

func (s *CachedUniversityCabinetService) Profile(ctx context.Context, vuzID string) (*model.University, error) {
	key := universityProfileCacheKey(vuzID)

	var cached model.University
	found, err := s.cache.GetJSON(ctx, key, &cached)
	if err == nil && found {
		return &cached, nil
	}
	if err != nil {
		s.logger.Warn("gateway cache read failed", "cache_key", key, "error", err)
	}

	university, err := s.next.Profile(ctx, vuzID)
	if err != nil {
		return nil, err
	}
	s.storeCache(ctx, key, university, s.cfg.UniversityProfileTTL)
	return university, nil
}

func (s *CachedUniversityCabinetService) ListBatches(ctx context.Context, vuzID string, limit int) ([]*model.Batch, error) {
	return s.next.ListBatches(ctx, vuzID, limit)
}

func (s *CachedDiplomaService) Upload(ctx context.Context, vuzID string, records []model.DiplomaUploadRecord) (*model.BatchUploadResponse, error) {
	response, err := s.next.Upload(ctx, vuzID, records)
	if err != nil {
		return nil, err
	}
	s.invalidateAdminStats(ctx)
	return response, nil
}

func (s *CachedDiplomaService) GetBatch(ctx context.Context, batchID, vuzID string) (*model.Batch, error) {
	key := batchStatusCacheKey(vuzID, batchID)

	var cached model.Batch
	found, err := s.cache.GetJSON(ctx, key, &cached)
	if err == nil && found {
		return &cached, nil
	}
	if err != nil {
		s.logger.Warn("gateway cache read failed", "cache_key", key, "error", err)
	}

	batch, err := s.next.GetBatch(ctx, batchID, vuzID)
	if err != nil {
		return nil, err
	}
	s.storeCache(ctx, key, batch, s.cfg.BatchStatusTTL)
	return batch, nil
}

func (s *CachedDiplomaService) DownloadBatch(ctx context.Context, batchID, vuzID string) ([]byte, error) {
	return s.next.DownloadBatch(ctx, batchID, vuzID)
}

func (s *CachedDiplomaService) Revoke(ctx context.Context, vuzID, hash string) error {
	if err := s.next.Revoke(ctx, vuzID, hash); err != nil {
		return err
	}
	s.invalidateAdminStats(ctx)
	return nil
}

func (s *CachedDiplomaService) HandleProcessingResult(ctx context.Context, result *model.KafkaProcessingResult) error {
	if err := s.next.HandleProcessingResult(ctx, result); err != nil {
		return err
	}

	s.invalidateAdminStats(ctx)

	if result != nil {
		if err := s.cache.Delete(ctx, batchStatusCacheKey(result.VUZID, result.BatchID)); err != nil {
			s.logger.Warn("gateway cache invalidate failed", "cache_key", batchStatusCacheKey(result.VUZID, result.BatchID), "error", err)
		}
	}

	return nil
}

func (s *CachedAdminService) invalidateUniversityCaches(ctx context.Context, universityID string) {
	keys := []string{
		adminStatsCacheKey(),
		adminUniversitiesCacheKey(),
		adminUniversityCacheKey(universityID),
		universityProfileCacheKey(universityID),
	}
	if err := s.cache.Delete(ctx, keys...); err != nil {
		s.logger.Warn("gateway cache invalidate failed", "university_id", universityID, "error", err)
	}
}

func (s *CachedDiplomaService) invalidateAdminStats(ctx context.Context) {
	if err := s.cache.Delete(ctx, adminStatsCacheKey()); err != nil {
		s.logger.Warn("gateway cache invalidate failed", "cache_key", adminStatsCacheKey(), "error", err)
	}
}

func (s *CachedAdminService) storeCache(ctx context.Context, key string, value any, ttl time.Duration) {
	if ttl <= 0 {
		return
	}
	if err := s.cache.SetJSON(ctx, key, value, ttl); err != nil {
		s.logger.Warn("gateway cache write failed", "cache_key", key, "error", err)
	}
}

func (s *CachedUniversityCabinetService) storeCache(ctx context.Context, key string, value any, ttl time.Duration) {
	if ttl <= 0 {
		return
	}
	if err := s.cache.SetJSON(ctx, key, value, ttl); err != nil {
		s.logger.Warn("gateway cache write failed", "cache_key", key, "error", err)
	}
}

func (s *CachedDiplomaService) storeCache(ctx context.Context, key string, value any, ttl time.Duration) {
	if ttl <= 0 {
		return
	}
	if err := s.cache.SetJSON(ctx, key, value, ttl); err != nil {
		s.logger.Warn("gateway cache write failed", "cache_key", key, "error", err)
	}
}

func adminStatsCacheKey() string {
	return "admin:stats"
}

func adminUniversitiesCacheKey() string {
	return "admin:universities"
}

func adminUniversityCacheKey(universityID string) string {
	return "admin:university:" + strings.TrimSpace(universityID)
}

func universityProfileCacheKey(universityID string) string {
	return "university:profile:" + strings.TrimSpace(universityID)
}

func batchStatusCacheKey(vuzID, batchID string) string {
	return fmt.Sprintf("diploma:batch:%s:%s", strings.TrimSpace(vuzID), strings.TrimSpace(batchID))
}
