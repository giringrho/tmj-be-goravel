package services

import (
	"errors"

	"github.com/goravel/framework/contracts/http"

	"goravel/app/facades"
	"goravel/app/models"
)

var (
	ErrEmailAlreadyExists = errors.New("email already registered")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrForbidden          = errors.New("forbidden")
)

type AuthService struct{}

func NewAuthService() *AuthService { return &AuthService{} }

// Register creates a new user with the given details. Default role is "user".
func (s *AuthService) Register(ctx http.Context, name, email, password string) (*models.User, string, error) {
	// Check email uniqueness.
	var existing models.User
	_ = facades.Orm().Query().Where("email = ?", email).First(&existing)
	if existing.ID > 0 {
		return nil, "", ErrEmailAlreadyExists
	}

	// Hash password.
	hashed, err := facades.Hash().Make(password)
	if err != nil {
		return nil, "", err
	}

	user := models.User{
		Name:     name,
		Email:    email,
		Password: hashed,
		Role:     "user",
	}
	if err := facades.Orm().Query().Create(&user); err != nil {
		return nil, "", err
	}

	// Generate JWT token.
	token, err := facades.Auth(ctx).LoginUsingID(user.ID)
	if err != nil {
		return nil, "", err
	}

	return &user, token, nil
}

// Login verifies credentials and returns the user + JWT token.
func (s *AuthService) Login(ctx http.Context, email, password string) (*models.User, string, error) {
	var user models.User
	_ = facades.Orm().Query().Where("email = ?", email).First(&user)
	if user.ID == 0 {
		return nil, "", ErrInvalidCredentials
	}

	if !facades.Hash().Check(password, user.Password) {
		return nil, "", ErrInvalidCredentials
	}

	token, err := facades.Auth(ctx).LoginUsingID(user.ID)
	if err != nil {
		return nil, "", err
	}

	return &user, token, nil
}
