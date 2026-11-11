// Package validation tidies and checks user input, with the same rules as the frontend.
package validation

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/Joshtri/rackup-api/internal/httpx"
)

// CleanName trims and collapses inner whitespace: "  Budi   Santoso " → "Budi Santoso".
// Same rule as `cleanName` in the frontend.
func CleanName(name string) string {
	return strings.Join(strings.Fields(name), " ")
}

// SameName compares names ignoring case, like `sameName` in the frontend.
func SameName(a, b string) bool {
	return strings.ToLower(a) == strings.ToLower(b)
}

// RequireText checks a cleaned, required text field's length in characters.
func RequireText(field, value string, maxChars int) error {
	if value == "" {
		return httpx.Validation(field + " is required")
	}
	if utf8.RuneCountInString(value) > maxChars {
		return httpx.Validation(fmt.Sprintf("%s must be at most %d characters", field, maxChars))
	}
	return nil
}
