package middleware

import (
	"strings"

	"github.com/goravel/framework/contracts/http"

	"goravel/app/facades"
	"goravel/app/models"
	"goravel/app/responses"
)

// contextKeyUser is the context key for the authenticated user.
type contextKeyUser struct{}

// ContextKeyUser returns the context key used to store/retrieve the
// authenticated user. Exported so controllers and services can access it.
func ContextKeyUser() any { return contextKeyUser{} }

// AuthMiddleware validates a JWT Bearer token, loads the user from the
// database, and stores it in the request context. If the token is missing
// or invalid, it aborts with 401.
type AuthMiddleware struct{}

func NewAuthMiddleware() *AuthMiddleware { return &AuthMiddleware{} }

func (m *AuthMiddleware) Signature() string { return "blog:auth" }

func (m *AuthMiddleware) Handle(ctx http.Context) {
	authHeader := ctx.Request().Header("Authorization")
	if authHeader == "" {
		responses.Unauthenticated(ctx)
		ctx.Request().Abort(http.StatusUnauthorized)
		return
	}

	// Expect "Bearer <token>"
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		responses.Unauthenticated(ctx)
		ctx.Request().Abort(http.StatusUnauthorized)
		return
	}

	token := parts[1]
	payload, err := facades.Auth(ctx).Parse(token)
	if err != nil {
		responses.Unauthenticated(ctx)
		ctx.Request().Abort(http.StatusUnauthorized)
		return
	}

	// Load user from DB using the ID from the JWT payload.
	var user models.User
	_ = facades.Orm().Query().Where("id = ?", payload.Key).First(&user)
	if user.ID == 0 {
		responses.Unauthenticated(ctx)
		ctx.Request().Abort(http.StatusUnauthorized)
		return
	}

	// Store user in context for downstream handlers.
	ctx.WithValue(ContextKeyUser(), &user)
	ctx.Request().Next()
}

// OptionalAuthMiddleware tries to parse a JWT Bearer token if present and
// loads the user into context. If the token is missing or invalid, it
// simply continues without aborting — useful for public endpoints that
// show different data based on whether the user is logged in (e.g. drafts).
type OptionalAuthMiddleware struct{}

func NewOptionalAuthMiddleware() *OptionalAuthMiddleware { return &OptionalAuthMiddleware{} }

func (m *OptionalAuthMiddleware) Signature() string { return "blog:optional-auth" }

func (m *OptionalAuthMiddleware) Handle(ctx http.Context) {
	authHeader := ctx.Request().Header("Authorization")
	if authHeader == "" {
		ctx.Request().Next()
		return
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		ctx.Request().Next()
		return
	}

	token := parts[1]
	payload, err := facades.Auth(ctx).Parse(token)
	if err != nil {
		ctx.Request().Next()
		return
	}

	var user models.User
	_ = facades.Orm().Query().Where("id = ?", payload.Key).First(&user)
	if user.ID == 0 {
		ctx.Request().Next()
		return
	}

	ctx.WithValue(ContextKeyUser(), &user)
	ctx.Request().Next()
}

// GetUser retrieves the authenticated user from the context, or nil.
func GetUser(ctx http.Context) *models.User {
	v := ctx.Value(ContextKeyUser())
	if v == nil {
		return nil
	}
	user, ok := v.(*models.User)
	if !ok {
		return nil
	}
	return user
}
