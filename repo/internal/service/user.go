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
	// BootstrapAdmin creates the first administrator from env vars if none exists.
	// It is a no-op when any active administrator already exists.
	BootstrapAdmin(ctx context.Context, username, email, password string) error
	// ChangePassword sets a new password and clears the must_rotate_password flag.
	ChangePassword(ctx context.Context, id uuid.UUID, newPassword string) error
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

// BootstrapAdmin creates the first ADMINISTRATOR account from the supplied
// credentials and marks it must_rotate_password=true. It is a no-op when any
// active administrator already exists.
func (s *userService) BootstrapAdmin(ctx context.Context, username, email, password string) error {
	count, err := s.userRepo.CountByRole(ctx, domain.RoleAdministrator)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil // at least one admin already exists
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	user := &domain.User{
		Username:           username,
		Email:              email,
		PasswordHash:       string(hash),
		Role:               domain.RoleAdministrator,
		IsActive:           true,
		MustRotatePassword: true,
	}
	created, err := s.userRepo.Create(ctx, user)
	if err != nil {
		return err
	}
	if s.auditSvc != nil {
		_ = s.auditSvc.Log(ctx, "users", created.ID, "BOOTSTRAP", nil, map[string]any{
			"username": created.Username,
			"role":     created.Role,
		})
	}
	return nil
}

// ChangePassword sets a new bcrypt password and clears the must_rotate_password flag.
func (s *userService) ChangePassword(ctx context.Context, id uuid.UUID, newPassword string) error {
	if len(newPassword) < 8 {
		return domain.NewValidationError("weak password", map[string]string{
			"password": "must be at least 8 characters",
		})
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	if err := s.userRepo.UpdatePassword(ctx, id, string(hash), true); err != nil {
		return err
	}
	if s.auditSvc != nil {
		_ = s.auditSvc.Log(ctx, "users", id, "CHANGE_PASSWORD", nil, nil)
	}
	return nil
}
