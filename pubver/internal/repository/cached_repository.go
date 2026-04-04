package repository

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"pubver/internal/domain"
	"pubver/internal/rediscache"
)

type CacheConfig struct {
	UniversityKeyTTL       time.Duration
	DiplomaRecordByHashTTL time.Duration
	DiplomaSearchResultTTL time.Duration
}

type CachedVerificationRepository struct {
	next   VerificationRepository
	cache  *rediscache.Client
	logger *slog.Logger
	cfg    CacheConfig
}

func NewCachedVerificationRepository(next VerificationRepository, cache *rediscache.Client, logger *slog.Logger, cfg CacheConfig) *CachedVerificationRepository {
	if logger == nil {
		logger = slog.Default()
	}

	return &CachedVerificationRepository{
		next:   next,
		cache:  cache,
		logger: logger,
		cfg:    cfg,
	}
}

func (r *CachedVerificationRepository) FindByHash(ctx context.Context, hash string) (*domain.DiplomaRecord, error) {
	key := "diploma_hash:" + strings.ToLower(strings.TrimSpace(hash))

	var cached domain.DiplomaRecord
	found, err := r.cache.GetJSON(ctx, key, &cached)
	if err == nil && found {
		return &cached, nil
	}
	if err != nil {
		r.logger.Warn("redis cache miss with backend error", "cache_key", key, "error", err)
	}

	record, err := r.next.FindByHash(ctx, hash)
	if err != nil {
		return nil, err
	}

	if err := r.cache.SetJSON(ctx, key, record, r.cfg.DiplomaRecordByHashTTL); err != nil {
		r.logger.Warn("store diploma record in cache", "cache_key", key, "error", err)
	}

	return record, nil
}

func (r *CachedVerificationRepository) FindByDiplomaNumber(ctx context.Context, vuzCode, diplomaNumber string) (*domain.DiplomaRecord, error) {
	key := "diploma_search:" + strings.ToLower(strings.TrimSpace(vuzCode)) + ":" + strings.TrimSpace(diplomaNumber)

	var cached domain.DiplomaRecord
	found, err := r.cache.GetJSON(ctx, key, &cached)
	if err == nil && found {
		return &cached, nil
	}
	if err != nil {
		r.logger.Warn("redis cache miss with backend error", "cache_key", key, "error", err)
	}

	record, err := r.next.FindByDiplomaNumber(ctx, vuzCode, diplomaNumber)
	if err != nil {
		return nil, err
	}

	if err := r.cache.SetJSON(ctx, key, record, r.cfg.DiplomaSearchResultTTL); err != nil {
		r.logger.Warn("store diploma search result in cache", "cache_key", key, "error", err)
	}

	return record, nil
}

func (r *CachedVerificationRepository) FindUniversityVerificationKeyByID(ctx context.Context, vuzID string) (*domain.UniversityVerificationKey, error) {
	key := "university_verification_key:" + strings.TrimSpace(vuzID)

	var cached domain.UniversityVerificationKey
	found, err := r.cache.GetJSON(ctx, key, &cached)
	if err == nil && found {
		return &cached, nil
	}
	if err != nil {
		r.logger.Warn("redis cache miss with backend error", "cache_key", key, "error", err)
	}

	verificationKey, err := r.next.FindUniversityVerificationKeyByID(ctx, vuzID)
	if err != nil {
		return nil, err
	}

	if err := r.cache.SetJSON(ctx, key, verificationKey, r.cfg.UniversityKeyTTL); err != nil {
		r.logger.Warn("store university verification key in cache", "cache_key", key, "error", err)
	}

	return verificationKey, nil
}
