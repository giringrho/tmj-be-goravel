package main

import (
	"goravel/bootstrap"
)

// @title           Tesmajoo Blog API (Goravel)
// @version         1.0
// @description     REST API for a Blog with JWT auth and RBAC (Soal 2 — Go).
// @host            localhost:3000
// @BasePath        /
// @securityDefinitions.apikey BearerAuth
// @in              header
// @name            Authorization
// @description     Enter JWT token with the `Bearer` prefix, e.g. "Bearer eyJhbGciOi..."
func main() {
	app := bootstrap.Boot()

	app.Start()
}
