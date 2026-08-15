package controllers

import (
	"errors"
	nethttp "net/http"
	"strconv"

	"github.com/goravel/framework/contracts/http"

	"goravel/app/http/middleware"
	"goravel/app/responses"
	"goravel/app/services"
)

type PostController struct {
	postService *services.PostService
}

func NewPostController() *PostController {
	return &PostController{
		postService: services.NewPostService(),
	}
}

// parsePostID extracts and validates the :id route parameter.
func parsePostID(ctx http.Context) (uint, bool) {
	idStr := ctx.Request().Route("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil || id == 0 {
		return 0, false
	}
	return uint(id), true
}

// List handles GET /api/posts
//
// @Summary      List posts
// @Description  Paginated list of posts. Published posts are visible to everyone; drafts only to the owner or admin.
// @Tags         Posts
// @Produce      json
// @Param        page   query  int  false  "Page number"   default(1)
// @Param        limit  query  int  false  "Items per page" default(15)
// @Success      200  {object}  object{data=[]models.Post,total=int,page=int,limit=int}
// @Failure      500  {object}  responses.ErrorResponse
// @Router       /api/posts [get]
func (c *PostController) List(ctx http.Context) http.Response {
	user := middleware.GetUser(ctx)
	currentUserID := uint(0)
	isAdmin := false
	if user != nil {
		currentUserID = user.ID
		isAdmin = user.Role == "admin"
	}

	page := ctx.Request().QueryInt("page", 1)
	limit := ctx.Request().QueryInt("limit", 15)

	posts, total, err := c.postService.ListPosts(currentUserID, isAdmin, page, limit)
	if err != nil {
		return responses.Internal(ctx, err.Error())
	}

	return ctx.Response().Success().Json(http.Json{
		"data":  posts,
		"total": total,
		"page":  page,
		"limit": limit,
	})
}

// Get handles GET /api/posts/:id
//
// @Summary      Get a post
// @Description  Return a single post by ID. Drafts are only visible to the owner or admin.
// @Tags         Posts
// @Produce      json
// @Param        id   path      int  true  "Post ID"
// @Success      200  {object}  object{data=models.Post}
// @Failure      400  {object}  responses.ErrorResponse
// @Failure      403  {object}  responses.ErrorResponse
// @Failure      404  {object}  responses.ErrorResponse
// @Router       /api/posts/{id} [get]
func (c *PostController) Get(ctx http.Context) http.Response {
	id, ok := parsePostID(ctx)
	if !ok {
		return responses.Simple(ctx, nethttp.StatusBadRequest, responses.CodeValidation, "invalid post id")
	}

	user := middleware.GetUser(ctx)
	currentUserID := uint(0)
	isAdmin := false
	if user != nil {
		currentUserID = user.ID
		isAdmin = user.Role == "admin"
	}

	post, err := c.postService.GetPost(id, currentUserID, isAdmin)
	if err != nil {
		if errors.Is(err, services.ErrPostNotFound) {
			return responses.NotFound(ctx, "post")
		}
		if errors.Is(err, services.ErrForbidden) {
			return responses.Forbidden(ctx)
		}
		return responses.Internal(ctx, err.Error())
	}

	return ctx.Response().Success().Json(http.Json{
		"data": post,
	})
}

// Create handles POST /api/posts
//
// @Summary      Create a post
// @Description  Create a new post. Requires role author or admin.
// @Tags         Posts
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        body  body      object{title=string,content=string,published=bool}  true  "Post payload"
// @Success      201   {object}  object{data=models.Post}
// @Failure      400   {object}  responses.ErrorResponse
// @Failure      401   {object}  responses.ErrorResponse
// @Failure      403   {object}  responses.ErrorResponse
// @Failure      500   {object}  responses.ErrorResponse
// @Router       /api/posts [post]
func (c *PostController) Create(ctx http.Context) http.Response {
	user := middleware.GetUser(ctx)
	if user == nil {
		return responses.Unauthenticated(ctx)
	}

	title := ctx.Request().Input("title")
	content := ctx.Request().Input("content")
	published := ctx.Request().InputBool("published")

	var details []responses.ErrorDetail
	if title == "" {
		details = append(details, responses.ErrorDetail{Field: "title", Message: "title is required"})
	}
	if content == "" {
		details = append(details, responses.ErrorDetail{Field: "content", Message: "content is required"})
	}
	if len(details) > 0 {
		return responses.JSON(ctx, nethttp.StatusBadRequest, responses.CodeValidation, "Validation failed", details)
	}

	post, err := c.postService.CreatePost(user.ID, title, content, published)
	if err != nil {
		return responses.Internal(ctx, err.Error())
	}

	return ctx.Response().Status(nethttp.StatusCreated).Json(http.Json{
		"data": post,
	})
}

// Update handles PUT /api/posts/:id
//
// @Summary      Update a post
// @Description  Update an existing post. Only the owner or admin can update.
// @Tags         Posts
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        id    path      int  true  "Post ID"
// @Param        body  body      object{title=string,content=string,published=bool}  true  "Fields to update"
// @Success      200   {object}  object{data=models.Post}
// @Failure      400   {object}  responses.ErrorResponse
// @Failure      401   {object}  responses.ErrorResponse
// @Failure      403   {object}  responses.ErrorResponse
// @Failure      404   {object}  responses.ErrorResponse
// @Router       /api/posts/{id} [put]
func (c *PostController) Update(ctx http.Context) http.Response {
	id, ok := parsePostID(ctx)
	if !ok {
		return responses.Simple(ctx, nethttp.StatusBadRequest, responses.CodeValidation, "invalid post id")
	}

	user := middleware.GetUser(ctx)
	if user == nil {
		return responses.Unauthenticated(ctx)
	}
	isAdmin := user.Role == "admin"

	// Build updates map from input.
	updates := make(map[string]any)
	if title := ctx.Request().Input("title"); title != "" {
		updates["title"] = title
	}
	if content := ctx.Request().Input("content"); content != "" {
		updates["content"] = content
	}
	// For boolean, we need to check if the field was actually provided.
	if ctx.Request().Input("published") != "" {
		updates["published"] = ctx.Request().InputBool("published")
	}

	if len(updates) == 0 {
		return responses.JSON(ctx, nethttp.StatusBadRequest, responses.CodeValidation,
			"No fields to update", []responses.ErrorDetail{
				{Field: "body", Message: "at least one field (title, content, published) must be provided"},
			})
	}

	post, err := c.postService.UpdatePost(id, user.ID, isAdmin, updates)
	if err != nil {
		if errors.Is(err, services.ErrPostNotFound) {
			return responses.NotFound(ctx, "post")
		}
		if errors.Is(err, services.ErrForbidden) {
			return responses.Forbidden(ctx)
		}
		return responses.Internal(ctx, err.Error())
	}

	return ctx.Response().Success().Json(http.Json{
		"data": post,
	})
}

// Delete handles DELETE /api/posts/:id
//
// @Summary      Delete a post
// @Description  Delete a post by ID. Only the owner or admin can delete.
// @Tags         Posts
// @Security     BearerAuth
// @Produce      json
// @Param        id  path  int  true  "Post ID"
// @Success      204
// @Failure      400  {object}  responses.ErrorResponse
// @Failure      401  {object}  responses.ErrorResponse
// @Failure      403  {object}  responses.ErrorResponse
// @Failure      404  {object}  responses.ErrorResponse
// @Router       /api/posts/{id} [delete]
func (c *PostController) Delete(ctx http.Context) http.Response {
	id, ok := parsePostID(ctx)
	if !ok {
		return responses.Simple(ctx, nethttp.StatusBadRequest, responses.CodeValidation, "invalid post id")
	}

	user := middleware.GetUser(ctx)
	if user == nil {
		return responses.Unauthenticated(ctx)
	}
	isAdmin := user.Role == "admin"

	err := c.postService.DeletePost(id, user.ID, isAdmin)
	if err != nil {
		if errors.Is(err, services.ErrPostNotFound) {
			return responses.NotFound(ctx, "post")
		}
		if errors.Is(err, services.ErrForbidden) {
			return responses.Forbidden(ctx)
		}
		return responses.Internal(ctx, err.Error())
	}

	return ctx.Response().NoContent(nethttp.StatusNoContent)
}
