package util_test

import (
	"testing"

	"github.com/fulfillops/fulfillops/internal/util"
)

func TestMaskPhone(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"5551234567", "***-***-4567"},
		{"555-123-4567", "***-***-4567"},
		{"(555) 123-4567", "***-***-4567"},
		{"1234567890", "***-***-7890"},
		{"123", "***-***-****"}, // too short
	}
	for _, c := range cases {
		got := util.MaskPhone(c.input)
		if got != c.want {
			t.Errorf("MaskPhone(%q) = %q; want %q", c.input, got, c.want)
		}
	}
}

func TestMaskEmail(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"jane@example.com", "j***@example.com"},
		{"a@b.com", "a***@b.com"},
		{"noemail", "***@***.***"},
		{"@domain.com", "***@***.***"},
	}
	for _, c := range cases {
		got := util.MaskEmail(c.input)
		if got != c.want {
			t.Errorf("MaskEmail(%q) = %q; want %q", c.input, got, c.want)
		}
	}
}

func TestMaskAddress(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"123 Main St", "123 ***"},
		{"456 Elm Avenue Apt 2B", "456 ***"},
		{"", "***"},
	}
	for _, c := range cases {
		got := util.MaskAddress(c.input)
		if got != c.want {
			t.Errorf("MaskAddress(%q) = %q; want %q", c.input, got, c.want)
		}
	}
}

func TestMaskVoucherCode(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"ABCDEFGH1234", "********1234"},
		{"1234", "****"},
		{"AB", "****"},
	}
	for _, c := range cases {
		got := util.MaskVoucherCode(c.input)
		if got != c.want {
			t.Errorf("MaskVoucherCode(%q) = %q; want %q", c.input, got, c.want)
		}
	}
}
