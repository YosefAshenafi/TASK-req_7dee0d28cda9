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

func TestMaskCity(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Boston", "B***"},
		{"New York", "N***"},
		{"", "***"},
	}
	for _, c := range cases {
		if got := util.MaskCity(c.in); got != c.want {
			t.Errorf("MaskCity(%q) = %q; want %q", c.in, got, c.want)
		}
	}
}

func TestMaskZip(t *testing.T) {
	cases := []struct{ in, want string }{
		{"02110", "021XX"},
		{"02110-1234", "021XX"},
		{"10001", "100XX"},
		{"12", "***XX"},
	}
	for _, c := range cases {
		if got := util.MaskZip(c.in); got != c.want {
			t.Errorf("MaskZip(%q) = %q; want %q", c.in, got, c.want)
		}
	}
}

func TestMaskState(t *testing.T) {
	for _, s := range []string{"MA", "NY", "CA"} {
		if got := util.MaskState(s); got != "**" {
			t.Errorf("MaskState(%q) = %q; want **", s, got)
		}
	}
}

func TestMaskShippingAddressForAuditor(t *testing.T) {
	l1, l2, city, state, zip := util.MaskShippingAddressForAuditor("123 Main St", "Apt 4", "Boston", "MA", "02110")
	if l1 != "123 ***" {
		t.Errorf("line1 = %q", l1)
	}
	if l2 != "Apt ***" {
		t.Errorf("line2 = %q", l2)
	}
	if city != "B***" {
		t.Errorf("city = %q", city)
	}
	if state != "**" {
		t.Errorf("state = %q", state)
	}
	if zip != "021XX" {
		t.Errorf("zip = %q", zip)
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
