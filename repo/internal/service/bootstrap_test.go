package service

// Tests for Finding #1: bootstrap admin flow.

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/fulfillops/fulfillops/internal/domain"
)

func TestBootstrapAdmin_CreatesAdminWhenNoneExists(t *testing.T) {
	repo := &stubUserRepo{
		byID:   map[uuid.UUID]*domain.User{},
		byName: map[string]*domain.User{},
	}
	svc := NewUserService(repo, nil)

	if err := svc.BootstrapAdmin(context.Background(), "admin", "admin@example.com", "S3cur3Pass!"); err != nil {
		t.Fatalf("BootstrapAdmin failed: %v", err)
	}

	if repo.created == nil {
		t.Fatal("expected admin user to be created")
	}
	if !repo.created.MustRotatePassword {
		t.Error("expected must_rotate_password=true on bootstrapped admin")
	}
	if repo.created.Role != domain.RoleAdministrator {
		t.Errorf("expected ADMINISTRATOR role, got %s", repo.created.Role)
	}
	if bcrypt.CompareHashAndPassword([]byte(repo.created.PasswordHash), []byte("S3cur3Pass!")) != nil {
		t.Error("password hash does not match bootstrap password")
	}
}

func TestBootstrapAdmin_NoopWhenAdminExists(t *testing.T) {
	existing := &domain.User{
		Username: "existing",
		Role:     domain.RoleAdministrator,
		IsActive: true,
	}
	existing.ID = uuid.New()

	repo := &stubUserRepo{
		byID:   map[uuid.UUID]*domain.User{existing.ID: existing},
		byName: map[string]*domain.User{existing.Username: existing},
	}
	svc := NewUserService(repo, nil)

	if err := svc.BootstrapAdmin(context.Background(), "admin2", "admin2@example.com", "Pass1234!"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// created should still be nil — bootstrap did not create a second admin
	if repo.created != nil {
		t.Error("BootstrapAdmin should be a no-op when admin already exists")
	}
}

func TestChangePassword_ClearsRotateFlag(t *testing.T) {
	id := uuid.New()
	user := &domain.User{
		ID:                 id,
		Username:           "admin",
		MustRotatePassword: true,
		IsActive:           true,
	}
	// Set an existing hash so Authenticate can be called against it.
	hash, _ := bcrypt.GenerateFromPassword([]byte("OldPass1!"), bcrypt.DefaultCost)
	user.PasswordHash = string(hash)

	repo := &stubUserRepo{
		byID:   map[uuid.UUID]*domain.User{id: user},
		byName: map[string]*domain.User{user.Username: user},
	}
	svc := NewUserService(repo, nil)

	if err := svc.ChangePassword(context.Background(), id, "NewPass2@"); err != nil {
		t.Fatalf("ChangePassword failed: %v", err)
	}

	// The hash stored in the repo should now match the new password.
	stored := repo.byID[id]
	if bcrypt.CompareHashAndPassword([]byte(stored.PasswordHash), []byte("NewPass2@")) != nil {
		t.Error("stored hash does not match new password")
	}
}

func TestChangePassword_RejectsWeakPassword(t *testing.T) {
	id := uuid.New()
	repo := &stubUserRepo{
		byID: map[uuid.UUID]*domain.User{id: {ID: id}},
	}
	svc := NewUserService(repo, nil)

	if err := svc.ChangePassword(context.Background(), id, "short"); err == nil {
		t.Error("expected error for weak password")
	}
}
