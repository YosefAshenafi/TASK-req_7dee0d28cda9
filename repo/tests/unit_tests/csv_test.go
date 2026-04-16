package unit_tests

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fulfillops/fulfillops/internal/util"
)

func TestWriteCSV(t *testing.T) {
	var buf bytes.Buffer
	headers := []string{"id", "name", "status"}
	rows := [][]string{
		{"1", "Alice", "ACTIVE"},
		{"2", "Bob", "INACTIVE"},
	}
	if err := util.WriteCSV(&buf, headers, rows); err != nil {
		t.Fatalf("WriteCSV: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "id,name,status") {
		t.Errorf("missing header row in CSV output:\n%s", out)
	}
	if !strings.Contains(out, "Alice") {
		t.Errorf("missing data row in CSV output:\n%s", out)
	}
	if !strings.Contains(out, "Bob") {
		t.Errorf("missing data row in CSV output:\n%s", out)
	}
}

func TestWriteCSVEmpty(t *testing.T) {
	var buf bytes.Buffer
	if err := util.WriteCSV(&buf, []string{"col"}, nil); err != nil {
		t.Fatalf("WriteCSV with no rows: %v", err)
	}
	if buf.String() != "col\n" {
		t.Errorf("expected header-only CSV, got %q", buf.String())
	}
}

func TestComputeFileChecksum(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	content := []byte("hello checksum")
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatal(err)
	}

	sum1, err := util.ComputeFileChecksum(path)
	if err != nil {
		t.Fatalf("ComputeFileChecksum: %v", err)
	}
	if len(sum1) != 64 { // SHA-256 hex = 64 chars
		t.Errorf("expected 64-char hex digest, got len=%d", len(sum1))
	}

	// Same file → same checksum.
	sum2, _ := util.ComputeFileChecksum(path)
	if sum1 != sum2 {
		t.Error("checksum is not deterministic")
	}
}

func TestComputeFileChecksumNonexistent(t *testing.T) {
	_, err := util.ComputeFileChecksum("/nonexistent/file.txt")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}
