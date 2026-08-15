package middleware

import (
	"github.com/goravel/framework/contracts/http"

	"goravel/app/responses"
)

// RoleMiddleware checks that the authenticated user has one of the allowed
// roles. It must run after AuthMiddleware. Use the Role() factory to create
// an instance with specific allowed roles.
type RoleMiddleware struct {
	allowedRoles []string
}

// Role creates a RoleMiddleware that allows the given roles.
// Usage: facades.Route().Middleware(middleware.Role("author", "admin"))
func Role(allowedRoles ...string) http.Middleware {
	return &RoleMiddleware{allowedRoles: allowedRoles}
}

func (m *RoleMiddleware) Signature() string { return "blog:role" }

func (m *RoleMiddleware) Handle(ctx http.Context) {
	user := GetUser(ctx)
	if user == nil {
		responses.Unauthenticated(ctx)
		ctx.Request().Abort(http.StatusUnauthorized)
		return
	}

	for _, role := range m.allowedRoles {
		if user.Role == role {
			ctx.Request().Next()
			return
		}
	}

	responses.Forbidden(ctx)
	ctx.Request().Abort(http.StatusForbidden)
}
