package routes

import (
	"github.com/goravel/framework/contracts/http"
	"github.com/goravel/framework/contracts/route"
	"github.com/goravel/framework/support"

	"goravel/app/facades"
	"goravel/app/http/controllers"
	"goravel/app/http/middleware"
)

func Web() {
	facades.Route().Get("/", func(ctx http.Context) http.Response {
		return ctx.Response().View().Make("welcome.tmpl", map[string]any{
			"version": support.Version,
		})
	})

	facades.Route().Static("public", "./public")

	userController := controllers.NewUserController()
	facades.Route().Get("/users", userController.Index)

	// Merchant CSV processing endpoints — wrapped with CORS middleware so the
	// Vite dev server can call them directly (needed for SSE streaming where
	// the Vite proxy buffers events and breaks real-time progress updates).
	corsMiddleware := middleware.NewCorsMiddleware()
	merchantController := controllers.NewMerchantController()
	facades.Route().Middleware(corsMiddleware).Group(func(r route.Router) {
		r.Post("/merchants/process", merchantController.Process)
		r.Post("/merchants/process/stream", merchantController.ProcessStream)
		r.Post("/merchants/process-dir", merchantController.ProcessDir)
	})
}
