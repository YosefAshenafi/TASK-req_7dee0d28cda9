package util

import (
	"strings"
)

// MaskPhone formats a phone number as ***-***-XXXX, showing only the last 4 digits.
// Input may be digits-only (e.g. "5551234567") or already formatted.
func MaskPhone(phone string) string {
	digits := extractDigits(phone)
	if len(digits) < 4 {
		return "***-***-****"
	}
	last4 := digits[len(digits)-4:]
	return "***-***-" + last4
}

// MaskEmail masks the local part of an email, showing only the first character.
// e.g. "jane@example.com" → "j***@example.com"
func MaskEmail(email string) string {
	at := strings.Index(email, "@")
	if at <= 0 {
		return "***@***.***"
	}
	local := email[:at]
	domain := email[at:]
	if len(local) == 0 {
		return "***" + domain
	}
	return string(local[0]) + "***" + domain
}

// MaskAddress masks most of a street address, showing only the house number.
// e.g. "123 Main St" → "123 ***"
func MaskAddress(address string) string {
	if address == "" {
		return "***"
	}
	parts := strings.Fields(address)
	if len(parts) == 0 {
		return "***"
	}
	return parts[0] + " ***"
}

// MaskVoucherCode shows only the last 4 characters of a voucher code.
// e.g. "ABCD-EFGH-1234" → "****-****-1234"
func MaskVoucherCode(code string) string {
	if len(code) <= 4 {
		return "****"
	}
	visible := code[len(code)-4:]
	masked := strings.Repeat("*", len(code)-4)
	return masked + visible
}

// extractDigits returns only the numeric characters from s.
func extractDigits(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
