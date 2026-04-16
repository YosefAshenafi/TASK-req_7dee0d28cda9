package util

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

// WriteCSV writes a CSV file with the given headers and rows to w.
// Each row must have the same number of elements as headers.
func WriteCSV(w io.Writer, headers []string, rows [][]string) error {
	cw := csv.NewWriter(w)
	if err := cw.Write(headers); err != nil {
		return fmt.Errorf("writing csv header: %w", err)
	}
	for _, row := range rows {
		if err := cw.Write(row); err != nil {
			return fmt.Errorf("writing csv row: %w", err)
		}
	}
	cw.Flush()
	return cw.Error()
}

// ComputeFileChecksum returns the hex-encoded SHA-256 digest of the file at path.
func ComputeFileChecksum(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("opening file for checksum: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("hashing file: %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
