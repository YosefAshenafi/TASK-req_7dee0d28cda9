package unit_tests

import (
	"testing"

	"github.com/fulfillops/fulfillops/internal/util"
)

func TestMaskPhone(t *testing.T) {
	cases := []struct{ input, want string }{
		{"5551234567", "***-***-4567"},
		{"555-123-4567", "***-***-4567"},
		{"(555) 123-4567", "***-***-4567"},
		{"1234567890", "***-***-7890"},
		{"123", "***-***-****"},   // fewer than 4 digits
		{"", "***-***-****"},      // empty
		{"abc", "***-***-****"},   // no digits
	}
	for _, c := range cases {
		got := util.MaskPhone(c.input)
		if got != c.want {
			t.Errorf("MaskPhone(%q) = %q; want %q", c.input, got, c.want)
		}
	}
}

func TestMaskEmail(t *testing.T) {
	cases := []struct{ input, want string }{
		{"jane@example.com", "j***@example.com"},
		{"a@b.com", "a***@b.com"},
		{"noemail", "***@***.***"},
		{"@domain.com", "***@***.***"},
		{"", "***@***.***"},
	}
	for _, c := range cases {
		got := util.MaskEmail(c.input)
		if got != c.want {
			t.Errorf("MaskEmail(%q) = %q; want %q", c.input, got, c.want)
		}
	}
}

func TestMaskAddress(t *testing.T) {
	cases := []struct{ input, want string }{
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

func TestMaskCity(t *testing.T) {
	cases := []struct{ input, want string }{
		{"Boston", "B***"},
		{"Springfield", "S***"},
		{"A", "A***"},
		{"", "***"},
	}
	for _, c := range cases {
		got := util.MaskCity(c.input)
		if got != c.want {
			t.Errorf("MaskCity(%q) = %q; want %q", c.input, got, c.want)
		}
	}
}

func TestMaskZip(t *testing.T) {
	cases := []struct{ input, want string }{
		{"02110", "021XX"},
		{"90210", "902XX"},
		{"02110-1234", "021XX"},
		{"12", "***XX"},
		{"", "***XX"},
	}
	for _, c := range cases {
		got := util.MaskZip(c.input)
		if got != c.want {
			t.Errorf("MaskZip(%q) = %q; want %q", c.input, got, c.want)
		}
	}
}

func TestMaskState(t *testing.T) {
	for _, s := range []string{"CA", "NY", "TX", "", "Illinois"} {
		got := util.MaskState(s)
		if got != "**" {
			t.Errorf("MaskState(%q) = %q; want **", s, got)
		}
	}
}

func TestMaskShippingAddressForAuditor(t *testing.T) {
	l1, l2, city, state, zip := util.MaskShippingAddressForAuditor(
		"123 Main St", "Apt 4B", "Boston", "MA", "02110",
	)
	if l1 != "123 ***" {
		t.Errorf("line1 = %q; want %q", l1, "123 ***")
	}
	if l2 != "Apt ***" {
		t.Errorf("line2 = %q; want %q", l2, "Apt ***")
	}
	if city != "B***" {
		t.Errorf("city = %q; want %q", city, "B***")
	}
	if state != "**" {
		t.Errorf("state = %q; want %q", state, "**")
	}
	if zip != "021XX" {
		t.Errorf("zip = %q; want %q", zip, "021XX")
	}

	// Empty line2 stays empty.
	_, l2Empty, _, _, _ := util.MaskShippingAddressForAuditor("456 Elm Ave", "", "Chicago", "IL", "60601")
	if l2Empty != "" {
		t.Errorf("empty line2 should stay empty, got %q", l2Empty)
	}
}

func TestMaskVoucherCode(t *testing.T) {
	cases := []struct{ input, want string }{
		{"ABCDEFGH1234", "********1234"},
		{"1234", "****"},
		{"AB", "****"},
		{"ABCD", "****"}, // exactly 4 chars → all masked
		{"12345", "*2345"},
	}
	for _, c := range cases {
		got := util.MaskVoucherCode(c.input)
		if got != c.want {
			t.Errorf("MaskVoucherCode(%q) = %q; want %q", c.input, got, c.want)
		}
	}
}
