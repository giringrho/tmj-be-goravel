package middleware

import (
	"github.com/goravel/framework/contracts/http"
)

// CorsMiddleware adds permissive CORS headers for local development so the
// Vite dev server (http://localhost:5173) can call the API directly (e.g. for
// SSE streaming where the Vite proxy buffers events).
type CorsMiddleware struct{}

func NewCorsMiddleware() *CorsMiddleware {
	return &CorsMiddleware{}
}

func (m *CorsMiddleware) Signature() string { return "cors" }

func (m *CorsMiddleware) Handle(ctx http.Context) {
	origin := ctx.Request().Header("Origin")
	if origin == "" {
		origin = "*"
	}

	// Set CORS headers via the framework API (works for JSON responses).
	ctx.Response().Header("Access-Control-Allow-Origin", origin)
	ctx.Response().Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, PATCH")
	ctx.Response().Header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
	ctx.Response().Header("Access-Control-Allow-Credentials", "true")
	ctx.Response().Header("Access-Control-Max-Age", "86400")

	// Also set on the underlying http.ResponseWriter so streaming responses
	// (which bypass Goravel's header chain) carry CORS headers too.
	w := ctx.Response().Writer()
	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, PATCH")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
	w.Header().Set("Access-Control-Allow-Credentials", "true")
	w.Header().Set("Access-Control-Max-Age", "86400")

	// Preflight: respond 204 immediately.
	if ctx.Request().Method() == "OPTIONS" {
		ctx.Response().Status(204)
		ctx.Request().Abort(204)
		return
	}

	ctx.Request().Next()
}
