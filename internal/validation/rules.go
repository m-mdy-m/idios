package validation

import (
	"fmt"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"
)

// Text validates a text field's emptiness, UTF-8 encoding, null bytes and size. Newline
// handling is intentionally left to the caller because descriptions and notes are multiline.
func Text(field, value string, required bool, maxRunes int) error {
	if !utf8.ValidString(value) || strings.ContainsRune(value, '\x00') {
		return fmt.Errorf("%s contains invalid text", field)
	}
	if required && strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s is required", field)
	}
	if maxRunes > 0 && utf8.RuneCountInString(value) > maxRunes {
		return fmt.Errorf("%s must be at most %d characters", field, maxRunes)
	}
	return nil
}

// ISODate accepts an empty value only for optional fields and otherwise requires a real
// calendar date encoded as YYYY-MM-DD.
func ISODate(field, value string, required bool) error {
	value = strings.TrimSpace(value)
	if value == "" {
		if required {
			return fmt.Errorf("%s is required", field)
		}
		return nil
	}
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil || parsed.Format("2006-01-02") != value {
		return fmt.Errorf("%s must use YYYY-MM-DD", field)
	}
	return nil
}

func HexColor(field, value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	if !validHexColor(value) {
		return fmt.Errorf("%s must be a hex color such as #64748b", field)
	}
	return nil
}

func URL(field, value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	parsed, err := url.ParseRequestURI(value)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return fmt.Errorf("%s must be a valid http:// or https:// URL", field)
	}
	return nil
}

func OneOf(field, value string, options ...string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	for _, option := range options {
		if value == option {
			return nil
		}
	}
	return fmt.Errorf("%s must be one of: %s", field, strings.Join(options, ", "))
}
