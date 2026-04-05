package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"net/netip"
	"runtime/debug"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (r *statusRecorder) WriteHeader(statusCode int) {
	r.statusCode = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

func (r *statusRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}

type RateLimitPolicy struct {
	Name           string
	RequestsPerSec float64
	Burst          int
}

type RateLimitConfig struct {
	Enabled           bool
	VerifyRPS         float64
	VerifyBurst       int
	SearchRPS         float64
	SearchBurst       int
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

type RateLimiter struct {
	logger            *slog.Logger
	redisClient       *redis.Client
	keyPrefix         string
	keyTTL            time.Duration
	trustedProxies    []netip.Prefix
	verifyPolicy      RateLimitPolicy
	searchPolicy      RateLimitPolicy
	tokenBucketScript *redis.Script
}

var redisTokenBucketScript = redis.NewScript(`
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
		logger:         logger,
		redisClient:    redisClient,
		keyPrefix:      cfg.Redis.KeyPrefix,
		keyTTL:         cfg.KeyTTL,
		trustedProxies: trustedProxies,
		verifyPolicy: RateLimitPolicy{
			Name:           "verify",
			RequestsPerSec: cfg.VerifyRPS,
			Burst:          cfg.VerifyBurst,
		},
		searchPolicy: RateLimitPolicy{
			Name:           "search",
			RequestsPerSec: cfg.SearchRPS,
			Burst:          cfg.SearchBurst,
		},
		tokenBucketScript: redisTokenBucketScript,
	}, nil
}

func (l *RateLimiter) Close() error {
	if l == nil || l.redisClient == nil {
		return nil
	}

	return l.redisClient.Close()
}

func withLogging(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(recorder, r)

		logger.Info(
			"http request",
			"request_id", requestIDFromContext(r.Context()),
			"method", r.Method,
			"path", r.URL.Path,
			"status", recorder.statusCode,
			"duration_ms", time.Since(start).Milliseconds(),
			"remote_addr", r.RemoteAddr,
		)
	})
}

func withRecover(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				logger.Error(
					"panic recovered",
					"request_id", requestIDFromContext(r.Context()),
					"panic", fmt.Sprint(recovered),
					"stack", string(debug.Stack()),
				)
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
			}
		}()

		next.ServeHTTP(w, r)
	})
}

func withRateLimit(limiter *RateLimiter, next http.Handler) http.Handler {
	if limiter == nil {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		policy, ok := limiter.policyForPath(r.URL.Path)
		if !ok {
			next.ServeHTTP(w, r)
			return
		}

		clientIP := limiter.extractClientIP(r)
		subjectKey := "ip:" + clientIP

		allowed, retryAfter, err := limiter.allow(r.Context(), policy, subjectKey)
		if err != nil {
			limiter.logger.Error("rate limit backend failed", "client_ip", clientIP, "path", r.URL.Path, "error", err)
			writeRateLimitUnavailable(w)
			return
		}
		if !allowed {
			limiter.logger.Warn("rate limit exceeded", "client_ip", clientIP, "path", r.URL.Path, "retry_after_sec", retryAfter, "policy", policy.Name)
			writeRateLimitExceeded(w, retryAfter)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func withRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = newRequestID()
		}

		w.Header().Set("X-Request-ID", requestID)
		next.ServeHTTP(w, r.WithContext(withRequestIDContext(r.Context(), requestID)))
	})
}

func (l *RateLimiter) policyForPath(path string) (RateLimitPolicy, bool) {
	switch path {
	case "/api/v1/verify":
		return l.verifyPolicy, true
	case "/api/v1/verify/search":
		return l.searchPolicy, true
	default:
		return RateLimitPolicy{}, false
	}
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
		candidates := splitAndTrimCommaSeparated(forwardedFor)
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

func writeRateLimitExceeded(w http.ResponseWriter, retryAfterSeconds int) {
	w.Header().Set("Retry-After", fmt.Sprintf("%d", retryAfterSeconds))
	writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "rate limit exceeded"})
}

func writeRateLimitUnavailable(w http.ResponseWriter) {
	writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "rate limiter unavailable"})
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

func splitAndTrimCommaSeparated(value string) []string {
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

func anyToInt(value any) (int, error) {
	switch typed := value.(type) {
	case int64:
		return int(typed), nil
	case int:
		return typed, nil
	case string:
		parsed := strings.TrimSpace(typed)
		if parsed == "" {
			return 0, fmt.Errorf("empty string in redis result")
		}
		var result int
		_, err := fmt.Sscanf(parsed, "%d", &result)
		if err != nil {
			return 0, fmt.Errorf("parse redis result %q: %w", typed, err)
		}
		return result, nil
	default:
		return 0, fmt.Errorf("unexpected redis result type %T", value)
	}
}

func newRequestID() string {
	buffer := make([]byte, 12)
	if _, err := rand.Read(buffer); err != nil {
		return fmt.Sprintf("fallback-%d", time.Now().UnixNano())
	}

	return hex.EncodeToString(buffer)
}
