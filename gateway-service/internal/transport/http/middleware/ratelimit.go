package middleware

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/netip"
	"strings"
	"time"

	"github.com/diasoft/gateway-service/internal/authctx"
	"github.com/redis/go-redis/v9"
)

type RateLimitConfig struct {
	Enabled           bool
	KeyTTL            time.Duration
	TrustedProxyCIDRs []string
	Redis             RedisConfig
}

type RedisConfig struct {
	Addr         string
	Password     string
	DB           int
	KeyPrefix    string
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

type RateLimitPolicy struct {
	Name           string
	RequestsPerSec float64
	Burst          int
}

type SubjectResolver func(r *http.Request, clientIP string) string

type RateLimiter struct {
	logger            *slog.Logger
	redisClient       *redis.Client
	keyPrefix         string
	keyTTL            time.Duration
	trustedProxies    []netip.Prefix
	tokenBucketScript *redis.Script
}

var tokenBucketScript = redis.NewScript(`
local key = KEYS[1]
local now = tonumber(ARGV[1])
local refill = tonumber(ARGV[2])
local burst = tonumber(ARGV[3])
local cost = tonumber(ARGV[4])
local ttl = tonumber(ARGV[5])

local values = redis.call("HMGET", key, "tokens", "ts")
local tokens = tonumber(values[1])
local ts = tonumber(values[2])

if tokens == nil then
  tokens = burst
end
if ts == nil then
  ts = now
end

if now > ts then
  local delta = now - ts
  tokens = math.min(burst, tokens + (delta * refill))
  ts = now
end

local allowed = 0
local retry_ms = 0

if tokens >= cost then
  tokens = tokens - cost
  allowed = 1
elseif refill > 0 then
  retry_ms = math.ceil((cost - tokens) / refill)
else
  retry_ms = ttl
end

redis.call("HMSET", key, "tokens", tokens, "ts", ts)
redis.call("PEXPIRE", key, ttl)

return {allowed, retry_ms}
`)

func NewRateLimiter(ctx context.Context, logger *slog.Logger, cfg RateLimitConfig) (*RateLimiter, error) {
	if logger == nil {
		logger = slog.Default()
	}
	if !cfg.Enabled {
		return nil, nil
	}
	if strings.TrimSpace(cfg.Redis.Addr) == "" {
		return nil, fmt.Errorf("RATE_LIMIT_REDIS_ADDR must be set when rate limiting is enabled")
	}
	if cfg.KeyTTL <= 0 {
		return nil, fmt.Errorf("RATE_LIMIT_KEY_TTL must be greater than zero when rate limiting is enabled")
	}

	trustedProxies, err := parseTrustedProxyCIDRs(cfg.TrustedProxyCIDRs)
	if err != nil {
		return nil, err
	}

	redisClient := redis.NewClient(&redis.Options{
		Addr:         cfg.Redis.Addr,
		Password:     cfg.Redis.Password,
		DB:           cfg.Redis.DB,
		DialTimeout:  cfg.Redis.DialTimeout,
		ReadTimeout:  cfg.Redis.ReadTimeout,
		WriteTimeout: cfg.Redis.WriteTimeout,
	})

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := redisClient.Ping(pingCtx).Err(); err != nil {
		_ = redisClient.Close()
		return nil, fmt.Errorf("connect rate limit redis: %w", err)
	}

	return &RateLimiter{
		logger:            logger,
		redisClient:       redisClient,
		keyPrefix:         cfg.Redis.KeyPrefix,
		keyTTL:            cfg.KeyTTL,
		trustedProxies:    trustedProxies,
		tokenBucketScript: tokenBucketScript,
	}, nil
}

func (l *RateLimiter) Close() error {
	if l == nil || l.redisClient == nil {
		return nil
	}
	return l.redisClient.Close()
}

func (l *RateLimiter) Middleware(policy RateLimitPolicy, resolver SubjectResolver) func(http.Handler) http.Handler {
	if l == nil {
		return func(next http.Handler) http.Handler { return next }
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			clientIP := l.extractClientIP(r)
			subject := resolver(r, clientIP)
			if strings.TrimSpace(subject) == "" {
				subject = "ip:" + clientIP
			}

			allowed, retryAfter, err := l.allow(r.Context(), policy, subject)
			if err != nil {
				l.logger.Error("rate limit backend failed", "policy", policy.Name, "subject", subject, "error", err)
				http.Error(w, "rate limiter unavailable", http.StatusServiceUnavailable)
				return
			}
			if !allowed {
				w.Header().Set("Retry-After", fmt.Sprintf("%d", retryAfter))
				http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func SubjectByClientIP(_ *http.Request, clientIP string) string {
	return "ip:" + clientIP
}

func SubjectByAdminOrIP(r *http.Request, clientIP string) string {
	if adminID, ok := authctx.AdminIDFromContext(r.Context()); ok && strings.TrimSpace(adminID) != "" {
		return "admin:" + adminID
	}
	return SubjectByClientIP(r, clientIP)
}

func SubjectByUniversityOrIP(r *http.Request, clientIP string) string {
	if universityID, ok := authctx.UniversityIDFromContext(r.Context()); ok && strings.TrimSpace(universityID) != "" {
		return "university:" + universityID
	}
	return SubjectByClientIP(r, clientIP)
}

func SubjectByAPIKeyOrUniversityOrIP(r *http.Request, clientIP string) string {
	if apiKeyID, ok := authctx.APIKeyIDFromContext(r.Context()); ok && strings.TrimSpace(apiKeyID) != "" {
		return "api_key:" + apiKeyID
	}
	return SubjectByUniversityOrIP(r, clientIP)
}

func (l *RateLimiter) allow(ctx context.Context, policy RateLimitPolicy, subjectKey string) (bool, int, error) {
	now := time.Now()
	key := fmt.Sprintf("%s:%s:%s", l.keyPrefix, policy.Name, subjectKey)
	refillPerMillisecond := policy.RequestsPerSec / 1000.0

	values, err := l.tokenBucketScript.Run(ctx, l.redisClient, []string{key},
		now.UnixMilli(),
		refillPerMillisecond,
		policy.Burst,
		1,
		l.keyTTL.Milliseconds(),
	).Result()
	if err != nil {
		return false, 0, err
	}

	resultSlice, ok := values.([]any)
	if !ok || len(resultSlice) != 2 {
		return false, 0, fmt.Errorf("unexpected redis rate limit response: %T", values)
	}

	allowed, err := anyToInt(resultSlice[0])
	if err != nil {
		return false, 0, err
	}
	retryAfterMS, err := anyToInt(resultSlice[1])
	if err != nil {
		return false, 0, err
	}

	retryAfterSec := 1
	if retryAfterMS > 1000 {
		retryAfterSec = int((retryAfterMS + 999) / 1000)
	}

	return allowed == 1, retryAfterSec, nil
}

func (l *RateLimiter) extractClientIP(r *http.Request) string {
	peerIP, ok := parseIP(r.RemoteAddr)
	if !ok {
		return strings.TrimSpace(r.RemoteAddr)
	}
	if !l.isTrustedProxy(peerIP) {
		return peerIP.String()
	}

	if forwardedFor := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); forwardedFor != "" {
		candidates := splitAndTrim(forwardedFor)
		for i := len(candidates) - 1; i >= 0; i-- {
			if candidateIP, ok := parseIP(candidates[i]); ok && !l.isTrustedProxy(candidateIP) {
				return candidateIP.String()
			}
		}
		if len(candidates) > 0 {
			if candidateIP, ok := parseIP(candidates[0]); ok {
				return candidateIP.String()
			}
		}
	}

	if realIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); realIP != "" {
		if candidateIP, ok := parseIP(realIP); ok {
			return candidateIP.String()
		}
	}

	return peerIP.String()
}

func (l *RateLimiter) isTrustedProxy(addr netip.Addr) bool {
	for _, prefix := range l.trustedProxies {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

func parseTrustedProxyCIDRs(values []string) ([]netip.Prefix, error) {
	if len(values) == 0 {
		return nil, nil
	}

	result := make([]netip.Prefix, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}

		if strings.Contains(trimmed, "/") {
			prefix, err := netip.ParsePrefix(trimmed)
			if err != nil {
				return nil, fmt.Errorf("parse trusted proxy CIDR %q: %w", trimmed, err)
			}
			result = append(result, prefix)
			continue
		}

		addr, err := netip.ParseAddr(trimmed)
		if err != nil {
			return nil, fmt.Errorf("parse trusted proxy IP %q: %w", trimmed, err)
		}
		result = append(result, netip.PrefixFrom(addr, addr.BitLen()))
	}

	return result, nil
}

func parseIP(value string) (netip.Addr, bool) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return netip.Addr{}, false
	}
	if addrPort, err := netip.ParseAddrPort(trimmed); err == nil {
		return addrPort.Addr(), true
	}

	addr, err := netip.ParseAddr(trimmed)
	if err != nil {
		return netip.Addr{}, false
	}
	return addr, true
}

func anyToInt(value any) (int, error) {
	switch typed := value.(type) {
	case int64:
		return int(typed), nil
	case int:
		return typed, nil
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return 0, fmt.Errorf("empty redis script result")
		}
		var result int
		if _, err := fmt.Sscanf(trimmed, "%d", &result); err != nil {
			return 0, fmt.Errorf("parse redis result %q: %w", typed, err)
		}
		return result, nil
	default:
		return 0, fmt.Errorf("unexpected redis result type %T", value)
	}
}

func splitAndTrim(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
