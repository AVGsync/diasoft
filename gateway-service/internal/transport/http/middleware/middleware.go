package middleware

import (
	"context"
	"database/sql"
	"net/http"
	"strings"

	"github.com/diasoft/gateway-service/internal/authctx"
	"github.com/diasoft/gateway-service/internal/infrastructure/token"
	"github.com/diasoft/gateway-service/internal/model"
)

type AccessTokenParser interface {
	ParseAccessToken(tokenString string) (*token.AccessClaims, error)
}

type APIKeyResolver interface {
	ResolveActiveUniversity(ctx context.Context, plainKey string) (*model.University, *model.APIKey, error)
}

type UniversityResolver interface {
	FindByID(ctx context.Context, id string) (*model.University, error)
}

type Middleware struct {
	tokens       AccessTokenParser
	apiKeys      APIKeyResolver
	universities UniversityResolver
}

func New(tokens AccessTokenParser, apiKeys APIKeyResolver, universities UniversityResolver) *Middleware {
	return &Middleware{
		tokens:       tokens,
		apiKeys:      apiKeys,
		universities: universities,
	}
}

func (m *Middleware) JWT(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bearerToken, ok := extractBearerToken(r)
		if !ok {
			http.Error(w, "missing bearer token", http.StatusUnauthorized)
			return
		}

		claims, err := m.tokens.ParseAccessToken(bearerToken)
		if err != nil {
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}

		switch claims.Role {
		case model.RoleAdmin:
			ctx := authctx.WithAdmin(r.Context(), claims.Subject, claims.Email)
			next.ServeHTTP(w, r.WithContext(ctx))
		case model.RoleUniversity:
			if claims.Status != model.UniversityStatusActive {
				http.Error(w, "university account is not active", http.StatusForbidden)
				return
			}

			universityID := claims.VUZID
			if universityID == "" {
				universityID = claims.Subject
			}
			if !m.ensureUniversityIsActive(w, r, universityID) {
				return
			}

			ctx := authctx.WithUniversity(r.Context(), universityID, claims.Email)
			next.ServeHTTP(w, r.WithContext(ctx))
		default:
			http.Error(w, "unsupported token role", http.StatusUnauthorized)
		}
	})
}

func (m *Middleware) Admin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !authctx.IsAdmin(r.Context()) {
			http.Error(w, "admin access required", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (m *Middleware) University(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !authctx.IsUniversity(r.Context()) {
			http.Error(w, "university access required", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (m *Middleware) UniversityOrAPIKey(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if bearerToken, ok := extractBearerToken(r); ok {
			claims, err := m.tokens.ParseAccessToken(bearerToken)
			if err == nil && claims.Role == model.RoleUniversity && claims.Status == model.UniversityStatusActive {
				universityID := claims.VUZID
				if universityID == "" {
					universityID = claims.Subject
				}
				if !m.ensureUniversityIsActive(w, r, universityID) {
					return
				}

				ctx := authctx.WithUniversity(r.Context(), universityID, claims.Email)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
		}

		apiKeyValue := extractAPIKey(r)
		if apiKeyValue == "" {
			http.Error(w, "missing credentials", http.StatusUnauthorized)
			return
		}

		university, apiKey, err := m.apiKeys.ResolveActiveUniversity(r.Context(), apiKeyValue)
		if err != nil {
			if err == sql.ErrNoRows {
				http.Error(w, "invalid api key", http.StatusUnauthorized)
				return
			}

			http.Error(w, "failed to resolve api key", http.StatusInternalServerError)
			return
		}

		ctx := authctx.WithUniversity(r.Context(), university.ID, university.Email)
		ctx = authctx.WithAPIKeyID(ctx, apiKey.ID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func extractBearerToken(r *http.Request) (string, bool) {
	header := r.Header.Get("Authorization")
	if header == "" {
		return "", false
	}

	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", false
	}

	return parts[1], true
}

func extractAPIKey(r *http.Request) string {
	if value := strings.TrimSpace(r.Header.Get("X-API-Key")); value != "" {
		return value
	}

	header := r.Header.Get("Authorization")
	if header == "" {
		return ""
	}

	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "ApiKey") {
		return ""
	}

	return parts[1]
}

func (m *Middleware) ensureUniversityIsActive(w http.ResponseWriter, r *http.Request, universityID string) bool {
	if m.universities == nil {
		return true
	}

	university, err := m.universities.FindByID(r.Context(), universityID)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "university not found", http.StatusUnauthorized)
			return false
		}

		http.Error(w, "failed to resolve university", http.StatusInternalServerError)
		return false
	}

	if university.Status != model.UniversityStatusActive {
		http.Error(w, "university account is not active", http.StatusForbidden)
		return false
	}

	return true
}
