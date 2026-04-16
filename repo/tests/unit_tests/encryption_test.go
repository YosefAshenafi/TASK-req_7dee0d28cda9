package unit_tests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/fulfillops/fulfillops/internal/service"
)

func writeTempKey(t *testing.T, content string, perm os.FileMode) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "encryption.key")
	if err := os.WriteFile(p, []byte(content), perm); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	p := writeTempKey(t, "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=\n", 0600)
	svc, err := service.NewEncryptionService(p)
	if err != nil {
		t.Fatalf("NewEncryptionService: %v", err)
	}

	cases := []string{
		"",
		"hello world",
		"jane@example.com",
		"unicode: こんにちは",
		"5551234567",
		"voucher-ABCD-1234",
	}
	for _, tc := range cases {
		enc, err := svc.EncryptString(tc)
		if err != nil {
			t.Errorf("Encrypt(%q): %v", tc, err)
			continue
		}
		dec, err := svc.DecryptToString(enc)
		if err != nil {
			t.Errorf("Decrypt(%q): %v", tc, err)
			continue
		}
		if dec != tc {
			t.Errorf("round-trip mismatch: got %q want %q", dec, tc)
		}
	}
}

func TestEncryptProducesUniqueCiphertexts(t *testing.T) {
	p := writeTempKey(t, "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=\n", 0600)
	svc, _ := service.NewEncryptionService(p)

	ct1, _ := svc.EncryptString("same input")
	ct2, _ := svc.EncryptString("same input")
	if string(ct1) == string(ct2) {
		t.Error("expected unique ciphertexts due to random nonce")
	}
}

func TestEncryptBytesRoundTrip(t *testing.T) {
	p := writeTempKey(t, "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=\n", 0600)
	svc, _ := service.NewEncryptionService(p)

	plain := []byte{0x00, 0xFF, 0x42, 0xAB}
	enc, err := svc.Encrypt(plain)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	dec, err := svc.Decrypt(enc)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if string(dec) != string(plain) {
		t.Errorf("byte round-trip mismatch")
	}
}

func TestNewEncryptionServiceErrors(t *testing.T) {
	t.Run("nonexistent_file", func(t *testing.T) {
		_, err := service.NewEncryptionService("/nonexistent/path/key")
		if err == nil {
			t.Error("expected error for nonexistent file")
		}
	})

	t.Run("wrong_permissions", func(t *testing.T) {
		p := writeTempKey(t, "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=\n", 0644)
		_, err := service.NewEncryptionService(p)
		if err == nil {
			t.Error("expected error for wrong file permissions (0644)")
		}
	})

	t.Run("too_short_key", func(t *testing.T) {
		p := writeTempKey(t, "dG9vc2hvcnQ=\n", 0600) // base64("tooshort") — only 8 bytes
		_, err := service.NewEncryptionService(p)
		if err == nil {
			t.Error("expected error for key shorter than 32 bytes")
		}
	})

	t.Run("invalid_base64", func(t *testing.T) {
		p := writeTempKey(t, "not-valid-base64!!!\n", 0600)
		_, err := service.NewEncryptionService(p)
		if err == nil {
			t.Error("expected error for invalid base64")
		}
	})
}
