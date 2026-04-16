package service

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// EncryptionService encrypts and decrypts data using AES-256-GCM.
type EncryptionService interface {
	Encrypt(plaintext []byte) ([]byte, error)
	Decrypt(ciphertext []byte) ([]byte, error)
	EncryptString(s string) ([]byte, error)
	DecryptToString(ciphertext []byte) (string, error)
}

type aesEncryptionService struct {
	key []byte
}

// NewEncryptionService reads a 32-byte base64-encoded key from keyPath.
// The file must be exactly 0600 permissions to prevent accidental exposure.
func NewEncryptionService(keyPath string) (EncryptionService, error) {
	info, err := os.Stat(keyPath)
	if err != nil {
		return nil, fmt.Errorf("encryption key file %q: %w", keyPath, err)
	}
	if info.Mode().Perm() != 0600 {
		return nil, fmt.Errorf("encryption key file %q has permissions %v; must be 0600", keyPath, info.Mode().Perm())
	}

	raw, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("reading encryption key: %w", err)
	}

	// Decode base64, trimming any trailing newline/whitespace.
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(raw)))
	if err != nil {
		return nil, fmt.Errorf("decoding encryption key (expected base64): %w", err)
	}
	if len(decoded) != 32 {
		return nil, fmt.Errorf("encryption key must be 32 bytes after base64 decode, got %d", len(decoded))
	}

	return &aesEncryptionService{key: decoded}, nil
}

// Encrypt encrypts plaintext using AES-256-GCM with a random 12-byte nonce.
// The returned ciphertext has the nonce prepended: [12 nonce bytes][ciphertext+tag].
func (s *aesEncryptionService) Encrypt(plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return nil, fmt.Errorf("creating AES cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("creating GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize()) // 12 bytes
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generating nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// Decrypt decrypts a ciphertext produced by Encrypt.
func (s *aesEncryptionService) Decrypt(ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return nil, fmt.Errorf("creating AES cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("creating GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}

	nonce, ct := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypting: %w", err)
	}
	return plaintext, nil
}

// EncryptString is a convenience wrapper for string inputs.
func (s *aesEncryptionService) EncryptString(str string) ([]byte, error) {
	return s.Encrypt([]byte(str))
}

// DecryptToString is a convenience wrapper that returns a string.
func (s *aesEncryptionService) DecryptToString(ciphertext []byte) (string, error) {
	b, err := s.Decrypt(ciphertext)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
