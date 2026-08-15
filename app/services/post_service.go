package services

import (
	"errors"

	"github.com/goravel/framework/contracts/database/orm"

	"goravel/app/facades"
	"goravel/app/models"
)

var (
	ErrPostNotFound = errors.New("post not found")
)

type PostService struct{}

func NewPostService() *PostService { return &PostService{} }

// ListPosts returns published posts (and drafts for the given user if any).
// When currentUserID > 0 and includeDrafts is true, drafts owned by the user
// (or all drafts if isAdmin) are also included.
func (s *PostService) ListPosts(currentUserID uint, isAdmin bool, page, limit int) ([]models.Post, int64, error) {
	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 15
	}

	// Build count query.
	countQuery := facades.Orm().Query().Model(&models.Post{})
	if currentUserID == 0 {
		countQuery = countQuery.Where("published = ?", true)
	} else if !isAdmin {
		countQuery = countQuery.Where("published = ? OR user_id = ?", true, currentUserID)
	}
	total, err := countQuery.Count()
	if err != nil {
		return nil, 0, err
	}

	// Build data query.
	query := facades.Orm().Query().With("User").Model(&models.Post{})
	if currentUserID == 0 {
		query = query.Where("published = ?", true)
	} else if !isAdmin {
		query = query.Where("published = ? OR user_id = ?", true, currentUserID)
	}

	var posts []models.Post
	if err := query.OrderByDesc("created_at").
		Offset((page - 1) * limit).Limit(limit).Get(&posts); err != nil {
		return nil, 0, err
	}

	return posts, total, nil
}

// GetPost returns a single post by ID. If the post is a draft, only the owner
// or admin can view it.
func (s *PostService) GetPost(id uint, currentUserID uint, isAdmin bool) (*models.Post, error) {
	var post models.Post
	_ = facades.Orm().Query().With("User").Where("id = ?", id).First(&post)
	if post.ID == 0 {
		return nil, ErrPostNotFound
	}

	// Draft visibility check.
	if !post.Published {
		if currentUserID == 0 {
			return nil, ErrPostNotFound
		}
		if post.UserID != currentUserID && !isAdmin {
			return nil, ErrForbidden
		}
	}

	return &post, nil
}

// CreatePost creates a new post owned by the given user.
func (s *PostService) CreatePost(userID uint, title, content string, published bool) (*models.Post, error) {
	post := models.Post{
		UserID:    userID,
		Title:     title,
		Content:   content,
		Published: published,
	}
	if err := facades.Orm().Query().Create(&post); err != nil {
		return nil, err
	}
	return &post, nil
}

// UpdatePost updates a post. Only the owner or admin can update.
func (s *PostService) UpdatePost(id uint, userID uint, isAdmin bool, updates map[string]any) (*models.Post, error) {
	post, err := s.GetPost(id, userID, isAdmin)
	if err != nil {
		return nil, err
	}

	if !isAdmin && post.UserID != userID {
		return nil, ErrForbidden
	}

	if title, ok := updates["title"]; ok {
		post.Title = title.(string)
	}
	if content, ok := updates["content"]; ok {
		post.Content = content.(string)
	}
	if published, ok := updates["published"]; ok {
		post.Published = published.(bool)
	}

	if err := facades.Orm().Query().Save(post); err != nil {
		return nil, err
	}
	return post, nil
}

// DeletePost deletes a post and all its comments + dataset in a transaction.
func (s *PostService) DeletePost(id uint, userID uint, isAdmin bool) error {
	post, err := s.GetPost(id, userID, isAdmin)
	if err != nil {
		return err
	}

	if !isAdmin && post.UserID != userID {
		return ErrForbidden
	}

	return facades.Orm().Transaction(func(tx orm.Query) error {
		// Delete comments.
		if _, err := tx.Where("post_id = ?", id).Delete(&models.Comment{}); err != nil {
			return err
		}
		// Delete post.
		if _, err := tx.Where("id = ?", id).Delete(&models.Post{}); err != nil {
			return err
		}
		return nil
	})
}

// CanModifyPost checks if the user can modify (update/delete) the post.
func (s *PostService) CanModifyPost(post *models.Post, userID uint, isAdmin bool) bool {
	return isAdmin || post.UserID == userID
}
