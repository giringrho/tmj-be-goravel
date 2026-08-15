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

type CommentController struct {
	commentService *services.CommentService
	postService    *services.PostService
}

func NewCommentController() *CommentController {
	return &CommentController{
		commentService: services.NewCommentService(),
		postService:    services.NewPostService(),
	}
}

func parseCommentID(ctx http.Context) (uint, bool) {
	idStr := ctx.Request().Route("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil || id == 0 {
		return 0, false
	}
	return uint(id), true
}

// ListByPost handles GET /api/posts/:id/comments
//
// @Summary      List comments by post
// @Description  Return all comments for a given post. Post visibility rules apply.
// @Tags         Comments
// @Produce      json
// @Param        id  path  int  true  "Post ID"
// @Success      200  {object}  object{data=[]models.Comment}
// @Failure      400  {object}  responses.ErrorResponse
// @Failure      403  {object}  responses.ErrorResponse
// @Failure      404  {object}  responses.ErrorResponse
// @Router       /api/posts/{id}/comments [get]
func (c *CommentController) ListByPost(ctx http.Context) http.Response {
	postID, ok := parsePostID(ctx)
	if !ok {
		return responses.Simple(ctx, nethttp.StatusBadRequest, responses.CodeValidation, "invalid post id")
	}

	// Check post visibility.
	user := middleware.GetUser(ctx)
	currentUserID := uint(0)
	isAdmin := false
	if user != nil {
		currentUserID = user.ID
		isAdmin = user.Role == "admin"
	}
	_, err := c.postService.GetPost(postID, currentUserID, isAdmin)
	if err != nil {
		if errors.Is(err, services.ErrPostNotFound) {
			return responses.NotFound(ctx, "post")
		}
		if errors.Is(err, services.ErrForbidden) {
			return responses.Forbidden(ctx)
		}
		return responses.Internal(ctx, err.Error())
	}

	comments, err := c.commentService.ListComments(postID)
	if err != nil {
		return responses.Internal(ctx, err.Error())
	}

	return ctx.Response().Success().Json(http.Json{
		"data": comments,
	})
}

// Create handles POST /api/posts/:id/comments
//
// @Summary      Create a comment
// @Description  Add a comment to a post. The post must be published, or the commenter is the owner/admin.
// @Tags         Comments
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        id    path      int  true  "Post ID"
// @Param        body  body      object{content=string}  true  "Comment payload"
// @Success      201   {object}  object{data=models.Comment}
// @Failure      400   {object}  responses.ErrorResponse
// @Failure      401   {object}  responses.ErrorResponse
// @Failure      403   {object}  responses.ErrorResponse
// @Failure      404   {object}  responses.ErrorResponse
// @Router       /api/posts/{id}/comments [post]
func (c *CommentController) Create(ctx http.Context) http.Response {
	postID, ok := parsePostID(ctx)
	if !ok {
		return responses.Simple(ctx, nethttp.StatusBadRequest, responses.CodeValidation, "invalid post id")
	}

	user := middleware.GetUser(ctx)
	if user == nil {
		return responses.Unauthenticated(ctx)
	}

	content := ctx.Request().Input("content")
	if content == "" {
		return responses.JSON(ctx, nethttp.StatusBadRequest, responses.CodeValidation,
			"Validation failed", []responses.ErrorDetail{
				{Field: "content", Message: "content is required"},
			})
	}

	comment, err := c.commentService.CreateComment(postID, user.ID, user.Role == "admin", content)
	if err != nil {
		if errors.Is(err, services.ErrPostNotFound) {
			return responses.NotFound(ctx, "post")
		}
		if errors.Is(err, services.ErrForbidden) {
			return responses.Forbidden(ctx)
		}
		return responses.Internal(ctx, err.Error())
	}

	return ctx.Response().Status(nethttp.StatusCreated).Json(http.Json{
		"data": comment,
	})
}

// Get handles GET /api/comments/:id
//
// @Summary      Get a comment
// @Description  Return a single comment by ID
// @Tags         Comments
// @Produce      json
// @Param        id  path  int  true  "Comment ID"
// @Success      200  {object}  object{data=models.Comment}
// @Failure      400  {object}  responses.ErrorResponse
// @Failure      404  {object}  responses.ErrorResponse
// @Router       /api/comments/{id} [get]
func (c *CommentController) Get(ctx http.Context) http.Response {
	id, ok := parseCommentID(ctx)
	if !ok {
		return responses.Simple(ctx, nethttp.StatusBadRequest, responses.CodeValidation, "invalid comment id")
	}

	comment, err := c.commentService.GetComment(id)
	if err != nil {
		return responses.NotFound(ctx, "comment")
	}

	return ctx.Response().Success().Json(http.Json{
		"data": comment,
	})
}

// Update handles PUT /api/comments/:id
//
// @Summary      Update a comment
// @Description  Update a comment's content. Only the owner or admin can update.
// @Tags         Comments
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        id    path      int  true  "Comment ID"
// @Param        body  body      object{content=string}  true  "Comment payload"
// @Success      200   {object}  object{data=models.Comment}
// @Failure      400   {object}  responses.ErrorResponse
// @Failure      401   {object}  responses.ErrorResponse
// @Failure      403   {object}  responses.ErrorResponse
// @Failure      404   {object}  responses.ErrorResponse
// @Router       /api/comments/{id} [put]
func (c *CommentController) Update(ctx http.Context) http.Response {
	id, ok := parseCommentID(ctx)
	if !ok {
		return responses.Simple(ctx, nethttp.StatusBadRequest, responses.CodeValidation, "invalid comment id")
	}

	user := middleware.GetUser(ctx)
	if user == nil {
		return responses.Unauthenticated(ctx)
	}
	isAdmin := user.Role == "admin"

	content := ctx.Request().Input("content")
	if content == "" {
		return responses.JSON(ctx, nethttp.StatusBadRequest, responses.CodeValidation,
			"Validation failed", []responses.ErrorDetail{
				{Field: "content", Message: "content is required"},
			})
	}

	comment, err := c.commentService.UpdateComment(id, user.ID, isAdmin, content)
	if err != nil {
		if errors.Is(err, services.ErrCommentNotFound) {
			return responses.NotFound(ctx, "comment")
		}
		if errors.Is(err, services.ErrForbidden) {
			return responses.Forbidden(ctx)
		}
		return responses.Internal(ctx, err.Error())
	}

	return ctx.Response().Success().Json(http.Json{
		"data": comment,
	})
}

// Delete handles DELETE /api/comments/:id
//
// @Summary      Delete a comment
// @Description  Delete a comment by ID. Only the owner or admin can delete.
// @Tags         Comments
// @Security     BearerAuth
// @Produce      json
// @Param        id  path  int  true  "Comment ID"
// @Success      204
// @Failure      400  {object}  responses.ErrorResponse
// @Failure      401  {object}  responses.ErrorResponse
// @Failure      403  {object}  responses.ErrorResponse
// @Failure      404  {object}  responses.ErrorResponse
// @Router       /api/comments/{id} [delete]
func (c *CommentController) Delete(ctx http.Context) http.Response {
	id, ok := parseCommentID(ctx)
	if !ok {
		return responses.Simple(ctx, nethttp.StatusBadRequest, responses.CodeValidation, "invalid comment id")
	}

	user := middleware.GetUser(ctx)
	if user == nil {
		return responses.Unauthenticated(ctx)
	}
	isAdmin := user.Role == "admin"

	err := c.commentService.DeleteComment(id, user.ID, isAdmin)
	if err != nil {
		if errors.Is(err, services.ErrCommentNotFound) {
			return responses.NotFound(ctx, "comment")
		}
		if errors.Is(err, services.ErrForbidden) {
			return responses.Forbidden(ctx)
		}
		return responses.Internal(ctx, err.Error())
	}

	return ctx.Response().NoContent(nethttp.StatusNoContent)
}
