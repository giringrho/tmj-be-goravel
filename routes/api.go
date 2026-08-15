package routes

import (
	"github.com/goravel/framework/contracts/route"

	"goravel/app/facades"
	"goravel/app/http/controllers"
	"goravel/app/http/middleware"
)

func Api() {
	authMiddleware := middleware.NewAuthMiddleware()
	optionalAuthMiddleware := middleware.NewOptionalAuthMiddleware()
	corsMiddleware := middleware.NewCorsMiddleware()

	authController := controllers.NewAuthController()
	postController := controllers.NewPostController()
	commentController := controllers.NewCommentController()
	adminUserController := controllers.NewAdminUserController()

	facades.Route().Middleware(corsMiddleware).Prefix("api").Group(func(router route.Router) {
		// Auth routes (public)
		router.Post("/auth/register", authController.Register)
		router.Post("/auth/login", authController.Login)

		// Authenticated auth routes
		router.Middleware(authMiddleware).Group(func(r route.Router) {
			r.Post("/auth/logout", authController.Logout)
			r.Get("/auth/me", authController.Me)
		})

		// Posts (public read, optional auth to show drafts for logged-in users)
		router.Middleware(optionalAuthMiddleware).Group(func(r route.Router) {
			r.Get("/posts", postController.List)
			r.Get("/posts/:id", postController.Get)

			// Comments list by post (public)
			r.Get("/posts/:id/comments", commentController.ListByPost)
		})

		// Authenticated routes
		router.Middleware(authMiddleware).Group(func(r route.Router) {
			// Create post: author + admin only
			r.Middleware(middleware.Role("author", "admin")).Post("/posts", postController.Create)

			// Update/Delete post: owner or admin
			r.Put("/posts/:id", postController.Update)
			r.Delete("/posts/:id", postController.Delete)

			// Comments
			r.Post("/posts/:id/comments", commentController.Create)
			r.Get("/comments/:id", commentController.Get)
			r.Put("/comments/:id", commentController.Update)
			r.Delete("/comments/:id", commentController.Delete)
		})

		// Admin routes
		router.Middleware(authMiddleware, middleware.Role("admin")).Group(func(r route.Router) {
			r.Get("/users", adminUserController.List)
			r.Put("/users/:id/role", adminUserController.ChangeRole)
		})
	})
}
