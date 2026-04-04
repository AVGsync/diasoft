package httpapi

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"net/netip"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
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

type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type ipRateLimiter struct {
	mu              sync.Mutex
	logger          *slog.Logger
	limit           rate.Limit
	burst           int
	visitorTTL      time.Duration
	cleanupInterval time.Duration
	lastCleanup     time.Time
	visitors        map[string]*visitor
}

func newIPRateLimiter(logger *slog.Logger, cfg RateLimitConfig) *ipRateLimiter {
	if logger == nil {
		logger = slog.Default()
	}

	return &ipRateLimiter{
		logger:          logger,
		limit:           rate.Limit(cfg.RequestsPerSec),
		burst:           cfg.Burst,
		visitorTTL:      cfg.VisitorTTL,
		cleanupInterval: cfg.CleanupInterval,
		visitors:        make(map[string]*visitor),
	}
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

func withRateLimit(logger *slog.Logger, cfg RateLimitConfig, next http.Handler) http.Handler {
	limiter := newIPRateLimiter(logger, cfg)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			next.ServeHTTP(w, r)
			return
		}

		clientIP := extractClientIP(r)
		now := time.Now()
		reservation := limiter.reserve(clientIP, now)
		if !reservation.OK() {
			logger.Warn("rate limit rejected", "client_ip", clientIP, "path", r.URL.Path)
			writeRateLimitExceeded(w, 1)
			return
		}

		delay := reservation.DelayFrom(now)
		if delay > 0 {
			reservation.CancelAt(now)
			retryAfter := int(math.Ceil(delay.Seconds()))
			if retryAfter < 1 {
				retryAfter = 1
			}
			logger.Warn("rate limit exceeded", "client_ip", clientIP, "path", r.URL.Path, "retry_after_sec", retryAfter)
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

func (l *ipRateLimiter) reserve(clientIP string, now time.Time) *rate.Reservation {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.cleanupVisitors(now)

	entry, ok := l.visitors[clientIP]
	if !ok {
		entry = &visitor{
			limiter: rate.NewLimiter(l.limit, l.burst),
		}
		l.visitors[clientIP] = entry
	}
	entry.lastSeen = now

	return entry.limiter.ReserveN(now, 1)
}

func (l *ipRateLimiter) cleanupVisitors(now time.Time) {
	if l.cleanupInterval <= 0 || l.visitorTTL <= 0 {
		return
	}
	if !l.lastCleanup.IsZero() && now.Sub(l.lastCleanup) < l.cleanupInterval {
		return
	}

	for clientIP, entry := range l.visitors {
		if now.Sub(entry.lastSeen) > l.visitorTTL {
			delete(l.visitors, clientIP)
		}
	}

	l.lastCleanup = now
}

func extractClientIP(r *http.Request) string {
	if forwardedFor := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); forwardedFor != "" {
		firstHop := strings.TrimSpace(strings.Split(forwardedFor, ",")[0])
		if firstHop != "" {
			return firstHop
		}
	}

	if realIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); realIP != "" {
		return realIP
	}

	host := strings.TrimSpace(r.RemoteAddr)
	if addr, err := netip.ParseAddrPort(host); err == nil {
		return addr.Addr().String()
	}
	if addr, err := netip.ParseAddr(host); err == nil {
		return addr.String()
	}

	return host
}

func writeRateLimitExceeded(w http.ResponseWriter, retryAfterSeconds int) {
	w.Header().Set("Retry-After", fmt.Sprintf("%d", retryAfterSeconds))
	writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "rate limit exceeded"})
}

func newRequestID() string {
	buffer := make([]byte, 12)
	if _, err := rand.Read(buffer); err != nil {
		return fmt.Sprintf("fallback-%d", time.Now().UnixNano())
	}

	return hex.EncodeToString(buffer)
}
