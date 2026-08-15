package controllers

import (
	nethttp "net/http"
	"strconv"

	"github.com/goravel/framework/contracts/http"

	"goravel/app/facades"
	"goravel/app/models"
	"goravel/app/responses"
)

type AdminUserController struct{}

func NewAdminUserController() *AdminUserController { return &AdminUserController{} }

// List handles GET /api/users (admin only)
//
// @Summary      List users
// @Description  Return all users. Admin only.
// @Tags         Users
// @Security     BearerAuth
// @Produce      json
// @Success      200  {object}  object{data=[]models.User}
// @Failure      401  {object}  responses.ErrorResponse
// @Failure      403  {object}  responses.ErrorResponse
// @Failure      500  {object}  responses.ErrorResponse
// @Router       /api/users [get]
func (c *AdminUserController) List(ctx http.Context) http.Response {
	var users []models.User
	err := facades.Orm().Query().OrderByDesc("created_at").Find(&users)
	if err != nil {
		return responses.Internal(ctx, err.Error())
	}

	return ctx.Response().Success().Json(http.Json{
		"data": users,
	})
}

// ChangeRole handles PUT /api/users/:id/role (admin only)
//
// @Summary      Change user role
// @Description  Update a user's role. Admin only. Valid roles: user, author, admin.
// @Tags         Users
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        id    path      int  true  "User ID"
// @Param        body  body      object{role=string}  true  "Role payload (user|author|admin)"
// @Success      200   {object}  object{data=models.User}
// @Failure      400   {object}  responses.ErrorResponse
// @Failure      401   {object}  responses.ErrorResponse
// @Failure      403   {object}  responses.ErrorResponse
// @Failure      404   {object}  responses.ErrorResponse
// @Router       /api/users/{id}/role [put]
func (c *AdminUserController) ChangeRole(ctx http.Context) http.Response {
	idStr := ctx.Request().Route("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil || id == 0 {
		return responses.Simple(ctx, nethttp.StatusBadRequest, responses.CodeValidation, "invalid user id")
	}

	role := ctx.Request().Input("role")
	if role == "" {
		return responses.JSON(ctx, nethttp.StatusBadRequest, responses.CodeValidation,
			"Validation failed", []responses.ErrorDetail{
				{Field: "role", Message: "role is required"},
			})
	}

	// Validate role value.
	validRoles := map[string]bool{"user": true, "author": true, "admin": true}
	if !validRoles[role] {
		return responses.JSON(ctx, nethttp.StatusBadRequest, responses.CodeValidation,
			"Validation failed", []responses.ErrorDetail{
				{Field: "role", Message: "role must be one of: user, author, admin"},
			})
	}

	var user models.User
	_ = facades.Orm().Query().Where("id = ?", id).First(&user)
	if user.ID == 0 {
		return responses.NotFound(ctx, "user")
	}

	user.Role = role
	if err := facades.Orm().Query().Save(&user); err != nil {
		return responses.Internal(ctx, err.Error())
	}

	return ctx.Response().Success().Json(http.Json{
		"data": user,
	})
}
