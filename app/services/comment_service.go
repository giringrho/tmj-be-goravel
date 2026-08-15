package services

import (
	"errors"

	"goravel/app/facades"
	"goravel/app/models"
)

var (
	ErrCommentNotFound = errors.New("comment not found")
)

type CommentService struct{}

func NewCommentService() *CommentService { return &CommentService{} }

// ListComments returns all comments for a post.
func (s *CommentService) ListComments(postID uint) ([]models.Comment, error) {
	var comments []models.Comment
	err := facades.Orm().Query().
		With("User").
		Where("post_id = ?", postID).
		OrderByDesc("created_at").
		Find(&comments)
	return comments, err
}

// GetComment returns a single comment by ID, with the associated user loaded.
// Post is loaded manually to avoid Goravel ORM eager-load issues with struct
// (non-pointer) relations.
func (s *CommentService) GetComment(id uint) (*models.Comment, error) {
	var comment models.Comment
	err := facades.Orm().Query().
		With("User").
		Where("id = ?", id).
		First(&comment)
	if err != nil {
		return nil, ErrCommentNotFound
	}
	// Manual load post.
	var post models.Post
	_ = facades.Orm().Query().Where("id = ?", comment.PostID).First(&post)
	comment.Post = post
	return &comment, nil
}

// CreateComment creates a comment on a post. The post must be published,
// or the commenter is the post owner / admin.
func (s *CommentService) CreateComment(postID, userID uint, isAdmin bool, content string) (*models.Comment, error) {
	// Verify post exists.
	var post models.Post
	_ = facades.Orm().Query().Where("id = ?", postID).First(&post)
	if post.ID == 0 {
		return nil, ErrPostNotFound
	}

	// Only allow comments on published posts (or if user owns the post / is admin).
	if !post.Published && post.UserID != userID && !isAdmin {
		return nil, ErrForbidden
	}

	comment := models.Comment{
		PostID:  postID,
		UserID:  userID,
		Content: content,
	}
	if err := facades.Orm().Query().Create(&comment); err != nil {
		return nil, err
	}
	return &comment, nil
}

// UpdateComment updates a comment. Only the owner or admin can update.
func (s *CommentService) UpdateComment(id, userID uint, isAdmin bool, content string) (*models.Comment, error) {
	comment, err := s.GetComment(id)
	if err != nil {
		return nil, err
	}

	if !isAdmin && comment.UserID != userID {
		return nil, ErrForbidden
	}

	comment.Content = content
	if err := facades.Orm().Query().Save(comment); err != nil {
		return nil, err
	}
	return comment, nil
}

// DeleteComment deletes a comment. The owner, the post owner, or admin can delete.
func (s *CommentService) DeleteComment(id, userID uint, isAdmin bool) error {
	comment, err := s.GetComment(id)
	if err != nil {
		return err
	}

	// Authorization: comment owner, post owner, or admin.
	isCommentOwner := comment.UserID == userID
	isPostOwner := comment.Post.UserID == userID
	if !isAdmin && !isCommentOwner && !isPostOwner {
		return ErrForbidden
	}

	_, err = facades.Orm().Query().Where("id = ?", id).Delete(&models.Comment{})
	return err
}
