package service

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/fulfillops/fulfillops/internal/domain"
	"github.com/fulfillops/fulfillops/internal/repository"
)

// UserService manages application users.
type UserService interface {
	Authenticate(ctx context.Context, username, password string) (*domain.User, error)
	CreateUser(ctx context.Context, username, email, password string, role domain.UserRole) (*domain.User, error)
	UpdateUser(ctx context.Context, id uuid.UUID, email string, role domain.UserRole) (*domain.User, error)
	DeactivateUser(ctx context.Context, id uuid.UUID) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
	List(ctx context.Context, role domain.UserRole, isActive *bool) ([]domain.User, error)
}

type userService struct {
	userRepo repository.UserRepository
	auditSvc AuditService
}

// NewUserService creates a UserService.
func NewUserService(userRepo repository.UserRepository, auditSvc AuditService) UserService {
	return &userService{userRepo: userRepo, auditSvc: auditSvc}
}

// Authenticate verifies credentials and returns the user on success.
func (s *userService) Authenticate(ctx context.Context, username, password string) (*domain.User, error) {
	user, err := s.userRepo.GetByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, domain.ErrUnauthorized
		}
		return nil, err
	}
	if !user.IsActive {
		return nil, domain.ErrUnauthorized
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, domain.ErrUnauthorized
	}
	return user, nil
}

// CreateUser hashes the password and persists a new user.
func (s *userService) CreateUser(ctx context.Context, username, email, password string, role domain.UserRole) (*domain.User, error) {
	if !role.IsValid() {
		return nil, domain.NewValidationError("invalid role", map[string]string{
			"role": "must be ADMINISTRATOR, FULFILLMENT_SPECIALIST, or AUDITOR",
		})
	}
	if len(password) < 8 {
		return nil, domain.NewValidationError("weak password", map[string]string{
			"password": "must be at least 8 characters",
		})
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	user := &domain.User{
		Username:     username,
		Email:        email,
		PasswordHash: string(hash),
		Role:         role,
		IsActive:     true,
	}

	created, err := s.userRepo.Create(ctx, user)
	if err != nil {
		return nil, err
	}

	if s.auditSvc != nil {
		_ = s.auditSvc.Log(ctx, "users", created.ID, "CREATE", nil, map[string]any{
			"username": created.Username,
			"role":     created.Role,
		})
	}
	return created, nil
}

// UpdateUser modifies a user's email and role.
func (s *userService) UpdateUser(ctx context.Context, id uuid.UUID, email string, role domain.UserRole) (*domain.User, error) {
	if !role.IsValid() {
		return nil, domain.NewValidationError("invalid role", map[string]string{
			"role": "must be ADMINISTRATOR, FULFILLMENT_SPECIALIST, or AUDITOR",
		})
	}

	user, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	before := *user

	user.Email = email
	user.Role = role

	updated, err := s.userRepo.Update(ctx, user)
	if err != nil {
		return nil, err
	}

	if s.auditSvc != nil {
		_ = s.auditSvc.Log(ctx, "users", updated.ID, "UPDATE", before, updated)
	}
	return updated, nil
}

// DeactivateUser marks a user as inactive.
func (s *userService) DeactivateUser(ctx context.Context, id uuid.UUID) error {
	if err := s.userRepo.Deactivate(ctx, id); err != nil {
		return err
	}
	if s.auditSvc != nil {
		_ = s.auditSvc.Log(ctx, "users", id, "DEACTIVATE", nil, nil)
	}
	return nil
}

func (s *userService) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	return s.userRepo.GetByID(ctx, id)
}

func (s *userService) List(ctx context.Context, role domain.UserRole, isActive *bool) ([]domain.User, error) {
	return s.userRepo.List(ctx, role, isActive)
}
