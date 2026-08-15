package bootstrap

import (
	contractsfoundation "github.com/goravel/framework/contracts/foundation"
	"github.com/goravel/framework/contracts/foundation/configuration"
	"github.com/goravel/framework/foundation"

	"goravel/app/http/middleware"
	"goravel/config"
	"goravel/routes"
)

func Boot() contractsfoundation.Application {
	return foundation.Setup().
		WithMigrations(Migrations).
		WithSeeders(Seeders).
		WithRouting(func() {
			routes.Web()
			routes.Api()
			routes.Grpc()
			routes.Swagger()
		}).
		WithMiddleware(func(m configuration.Middleware) {
			// CORS is needed so the Vite dev server (http://localhost:5173)
			// can call the API directly — in particular for SSE streaming
			// where the Vite proxy buffers events and breaks real-time
			// progress updates.
			m.Append(middleware.NewCorsMiddleware())
		}).
		WithProviders(Providers).
		WithCommands(Commands).
		WithConfig(config.Boot).
		Create()
}
