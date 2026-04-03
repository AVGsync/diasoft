package authctx

import "context"

type ctxKey string

const (
	roleKey         ctxKey = "role"
	universityIDKey ctxKey = "universityID"
	adminIDKey      ctxKey = "adminID"
	emailKey        ctxKey = "email"
	apiKeyIDKey     ctxKey = "apiKeyID"
)

func WithUniversity(ctx context.Context, universityID, email string) context.Context {
	ctx = context.WithValue(ctx, roleKey, "university")
	ctx = context.WithValue(ctx, universityIDKey, universityID)
	ctx = context.WithValue(ctx, emailKey, email)
	return ctx
}

func WithAdmin(ctx context.Context, adminID, email string) context.Context {
	ctx = context.WithValue(ctx, roleKey, "admin")
	ctx = context.WithValue(ctx, adminIDKey, adminID)
	ctx = context.WithValue(ctx, emailKey, email)
	return ctx
}

func WithAPIKeyID(ctx context.Context, apiKeyID string) context.Context {
	return context.WithValue(ctx, apiKeyIDKey, apiKeyID)
}

func RoleFromContext(ctx context.Context) (string, bool) {
	value, ok := ctx.Value(roleKey).(string)
	return value, ok
}

func UniversityIDFromContext(ctx context.Context) (string, bool) {
	value, ok := ctx.Value(universityIDKey).(string)
	return value, ok
}

func AdminIDFromContext(ctx context.Context) (string, bool) {
	value, ok := ctx.Value(adminIDKey).(string)
	return value, ok
}

func EmailFromContext(ctx context.Context) (string, bool) {
	value, ok := ctx.Value(emailKey).(string)
	return value, ok
}

func APIKeyIDFromContext(ctx context.Context) (string, bool) {
	value, ok := ctx.Value(apiKeyIDKey).(string)
	return value, ok
}

func IsUniversity(ctx context.Context) bool {
	role, ok := RoleFromContext(ctx)
	return ok && role == "university"
}

func IsAdmin(ctx context.Context) bool {
	role, ok := RoleFromContext(ctx)
	return ok && role == "admin"
}
