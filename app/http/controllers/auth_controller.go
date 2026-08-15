package controllers

import (
	"errors"
	nethttp "net/http"

	"github.com/goravel/framework/contracts/http"

	"goravel/app/http/middleware"
	"goravel/app/responses"
	"goravel/app/services"
)

type AuthController struct {
	authService *services.AuthService
}

func NewAuthController() *AuthController {
	return &AuthController{authService: services.NewAuthService()}
}

// Register handles POST /api/auth/register
//
// @Summary      Register a new user
// @Description  Create a new user account (default role: user) and return a JWT token
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        body  body      object{name=string,email=string,password=string,password_confirmation=string}  true  "Register payload"
// @Success      201   {object}  object{token=string,user=models.User}
// @Failure      400   {object}  responses.ErrorResponse
// @Failure      409   {object}  responses.ErrorResponse
// @Failure      500   {object}  responses.ErrorResponse
// @Router       /api/auth/register [post]
func (c *AuthController) Register(ctx http.Context) http.Response {
	name := ctx.Request().Input("name")
	email := ctx.Request().Input("email")
	password := ctx.Request().Input("password")
	passwordConfirmation := ctx.Request().Input("password_confirmation")

	// Basic validation
	var details []responses.ErrorDetail
	if name == "" {
		details = append(details, responses.ErrorDetail{Field: "name", Message: "name is required"})
	}
	if email == "" {
		details = append(details, responses.ErrorDetail{Field: "email", Message: "email is required"})
	}
	if password == "" {
		details = append(details, responses.ErrorDetail{Field: "password", Message: "password is required"})
	}
	if len(password) < 8 {
		details = append(details, responses.ErrorDetail{Field: "password", Message: "password must be at least 8 characters"})
	}
	if password != passwordConfirmation {
		details = append(details, responses.ErrorDetail{Field: "password", Message: "password confirmation does not match"})
	}
	if len(details) > 0 {
		return responses.JSON(ctx, nethttp.StatusBadRequest, responses.CodeValidation, "Validation failed", details)
	}

	user, token, err := c.authService.Register(ctx, name, email, password)
	if err != nil {
		if errors.Is(err, services.ErrEmailAlreadyExists) {
			return responses.Conflict(ctx, "email already registered")
		}
		return responses.Internal(ctx, err.Error())
	}

	return ctx.Response().Status(nethttp.StatusCreated).Json(http.Json{
		"token": token,
		"user":  user,
	})
}

// Login handles POST /api/auth/login
//
// @Summary      Login
// @Description  Authenticate a user and return a JWT token
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        body  body  object{email=string,password=string}  true  "Login payload"
// @Success      200   {object}  object{token=string,user=models.User}
// @Failure      400   {object}  responses.ErrorResponse
// @Failure      401   {object}  responses.ErrorResponse
// @Failure      500   {object}  responses.ErrorResponse
// @Router       /api/auth/login [post]
func (c *AuthController) Login(ctx http.Context) http.Response {
	email := ctx.Request().Input("email")
	password := ctx.Request().Input("password")

	var details []responses.ErrorDetail
	if email == "" {
		details = append(details, responses.ErrorDetail{Field: "email", Message: "email is required"})
	}
	if password == "" {
		details = append(details, responses.ErrorDetail{Field: "password", Message: "password is required"})
	}
	if len(details) > 0 {
		return responses.JSON(ctx, nethttp.StatusBadRequest, responses.CodeValidation, "Validation failed", details)
	}

	user, token, err := c.authService.Login(ctx, email, password)
	if err != nil {
		if errors.Is(err, services.ErrInvalidCredentials) {
			return responses.Simple(ctx, nethttp.StatusUnauthorized, responses.CodeUnauthenticated, "invalid credentials")
		}
		return responses.Internal(ctx, err.Error())
	}

	return ctx.Response().Success().Json(http.Json{
		"token": token,
		"user":  user,
	})
}

// Logout handles POST /api/auth/logout
//
// @Summary      Logout
// @Description  Invalidate the current JWT token (stateless — client discards token)
// @Tags         Auth
// @Security     BearerAuth
// @Produce      json
// @Success      200  {object}  object{message=string}
// @Router       /api/auth/logout [post]
func (c *AuthController) Logout(ctx http.Context) http.Response {
	// JWT is stateless; client discards the token.
	// We could blacklist the token here if needed.
	return ctx.Response().Success().Json(http.Json{
		"message": "logged out successfully",
	})
}

// Me handles GET /api/auth/me
//
// @Summary      Get current user
// @Description  Return the authenticated user's profile
// @Tags         Auth
// @Security     BearerAuth
// @Produce      json
// @Success      200  {object}  object{user=models.User}
// @Failure      401  {object}  responses.ErrorResponse
// @Router       /api/auth/me [get]
func (c *AuthController) Me(ctx http.Context) http.Response {
	user := middleware.GetUser(ctx)
	if user == nil {
		return responses.Unauthenticated(ctx)
	}
	return ctx.Response().Success().Json(http.Json{
		"user": user,
	})
}
