package service_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/fulfillops/fulfillops/internal/service"
)

func writeTempKey(t *testing.T, content string, perm os.FileMode) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "encryption.key")
	if err := os.WriteFile(p, []byte(content), perm); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	// 32 random bytes base64-encoded
	keyB64 := "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=" // 32 zero bytes, valid base64
	p := writeTempKey(t, keyB64+"\n", 0600)

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
		encrypted, err := svc.EncryptString(tc)
		if err != nil {
			t.Errorf("Encrypt(%q): %v", tc, err)
			continue
		}
		decrypted, err := svc.DecryptToString(encrypted)
		if err != nil {
			t.Errorf("Decrypt(%q): %v", tc, err)
			continue
		}
		if decrypted != tc {
			t.Errorf("round-trip mismatch: got %q want %q", decrypted, tc)
		}
	}
}

func TestEncryptProducesUniqueCiphertexts(t *testing.T) {
	keyB64 := "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="
	p := writeTempKey(t, keyB64+"\n", 0600)
	svc, _ := service.NewEncryptionService(p)

	ct1, _ := svc.EncryptString("same input")
	ct2, _ := svc.EncryptString("same input")
	if string(ct1) == string(ct2) {
		t.Error("expected unique ciphertexts due to random nonce")
	}
}

func TestNewEncryptionServiceErrors(t *testing.T) {
	t.Run("nonexistent file", func(t *testing.T) {
		_, err := service.NewEncryptionService("/nonexistent/path/key")
		if err == nil {
			t.Error("expected error for nonexistent file")
		}
	})

	t.Run("wrong permissions", func(t *testing.T) {
		keyB64 := "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="
		p := writeTempKey(t, keyB64+"\n", 0644) // wrong perms
		_, err := service.NewEncryptionService(p)
		if err == nil {
			t.Error("expected error for wrong file permissions")
		}
	})

	t.Run("too short key", func(t *testing.T) {
		p := writeTempKey(t, "dG9vc2hvcnQ=\n", 0600) // base64("tooshort")
		_, err := service.NewEncryptionService(p)
		if err == nil {
			t.Error("expected error for short key")
		}
	})

	t.Run("invalid base64", func(t *testing.T) {
		p := writeTempKey(t, "not-valid-base64!!!\n", 0600)
		_, err := service.NewEncryptionService(p)
		if err == nil {
			t.Error("expected error for invalid base64")
		}
	})
}
