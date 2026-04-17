package handler

// Unit test for Issue 3: customer partial update must preserve encrypted
// fields. A name-only update with omitted phone/email/address keys must NOT
// null out the existing ciphertext. This is the regression test the audit
// flagged.

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/fulfillops/fulfillops/internal/domain"
	"github.com/fulfillops/fulfillops/internal/service"
)

// recordingCustomerRepo tracks the row passed to Update so assertions can
// confirm the encrypted-field values after a partial update.
type recordingCustomerRepo struct {
	before  *domain.Customer
	updated *domain.Customer
}

func (r *recordingCustomerRepo) List(context.Context, string, domain.PageRequest, bool) ([]domain.Customer, int, error) {
	return nil, 0, nil
}

func (r *recordingCustomerRepo) GetByID(_ context.Context, id uuid.UUID) (*domain.Customer, error) {
	if r.before == nil {
		return nil, domain.NewNotFoundError("customer")
	}
	c := *r.before
	c.ID = id
	return &c, nil
}

func (r *recordingCustomerRepo) Create(context.Context, *domain.Customer) (*domain.Customer, error) {
	return nil, nil
}

func (r *recordingCustomerRepo) Update(_ context.Context, c *domain.Customer) (*domain.Customer, error) {
	copy := *c
	r.updated = &copy
	c.Version++
	return c, nil
}

func (r *recordingCustomerRepo) SoftDelete(context.Context, uuid.UUID, uuid.UUID) error {
	return nil
}
func (r *recordingCustomerRepo) Restore(context.Context, uuid.UUID) error { return nil }

// identityEncryptionSvc round-trips plaintext as ciphertext prefix so assertions
// can compare equality of stored "ciphertext" bytes across calls.
type identityEncryptionSvc struct{}

func (identityEncryptionSvc) Encrypt(p []byte) ([]byte, error) {
	out := append([]byte{0x01}, p...)
	return out, nil
}
func (identityEncryptionSvc) Decrypt(c []byte) ([]byte, error) {
	if len(c) > 0 && c[0] == 0x01 {
		return c[1:], nil
	}
	return c, nil
}
func (s identityEncryptionSvc) EncryptString(p string) ([]byte, error) { return s.Encrypt([]byte(p)) }
func (s identityEncryptionSvc) DecryptToString(c []byte) (string, error) {
	b, err := s.Decrypt(c)
	return string(b), err
}

// Ensure identityEncryptionSvc satisfies the interface used by the handler.
var _ service.EncryptionService = identityEncryptionSvc{}

func TestCustomerHandler_Update_NameOnlyPreservesEncryptedFields(t *testing.T) {
	gin.SetMode(gin.TestMode)

	id := uuid.New()
	existing := &domain.Customer{
		ID:               id,
		Name:             "Original",
		PhoneEncrypted:   []byte{0x01, 'p', 'h', 'o', 'n', 'e'},
		EmailEncrypted:   []byte{0x01, 'e', 'm', 'a', 'i', 'l'},
		AddressEncrypted: []byte{0x01, 'a', 'd', 'd', 'r'},
		Version:          1,
	}
	repo := &recordingCustomerRepo{before: existing}
	h := NewCustomerHandler(repo, identityEncryptionSvc{})

	r := gin.New()
	r.PUT("/customers/:id", h.Update)

	// Name-only partial update — client sends NO phone/email/address keys.
	payload := map[string]any{"name": "Renamed", "version": 1}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPut, "/customers/"+id.String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d\nbody: %s", rec.Code, rec.Body.String())
	}
	if repo.updated == nil {
		t.Fatal("expected Update to be invoked")
	}
	if repo.updated.Name != "Renamed" {
		t.Errorf("name not updated: %q", repo.updated.Name)
	}
	// Critical assertion — encrypted fields must survive a name-only update.
	if !bytes.Equal(repo.updated.PhoneEncrypted, existing.PhoneEncrypted) {
		t.Errorf("PhoneEncrypted wiped: %v", repo.updated.PhoneEncrypted)
	}
	if !bytes.Equal(repo.updated.EmailEncrypted, existing.EmailEncrypted) {
		t.Errorf("EmailEncrypted wiped: %v", repo.updated.EmailEncrypted)
	}
	if !bytes.Equal(repo.updated.AddressEncrypted, existing.AddressEncrypted) {
		t.Errorf("AddressEncrypted wiped: %v", repo.updated.AddressEncrypted)
	}
}

func TestCustomerHandler_Update_ExplicitEmptyStringClearsField(t *testing.T) {
	gin.SetMode(gin.TestMode)

	id := uuid.New()
	existing := &domain.Customer{
		ID:             id,
		Name:           "Original",
		PhoneEncrypted: []byte{0x01, 'p', 'h', 'o', 'n', 'e'},
		EmailEncrypted: []byte{0x01, 'e', 'm', 'a', 'i', 'l'},
		Version:        1,
	}
	repo := &recordingCustomerRepo{before: existing}
	h := NewCustomerHandler(repo, identityEncryptionSvc{})

	r := gin.New()
	r.PUT("/customers/:id", h.Update)

	// Explicit empty strings — the caller wants to clear those fields. A nil
	// pointer means "don't touch"; an empty string means "delete it".
	payload := map[string]any{"name": "Renamed", "version": 1, "phone": "", "email": ""}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPut, "/customers/"+id.String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if len(repo.updated.PhoneEncrypted) != 0 {
		t.Errorf("expected phone cleared, got %v", repo.updated.PhoneEncrypted)
	}
	if len(repo.updated.EmailEncrypted) != 0 {
		t.Errorf("expected email cleared, got %v", repo.updated.EmailEncrypted)
	}
}
