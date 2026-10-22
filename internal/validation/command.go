// Package validation contains the shared, deterministic validation flow used before
// protocol commands reach application use-cases. It deliberately depends on no IDIOS
// layer so that CLI, TUI, HTTP and future adapters can share the same rules.
package validation

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

type Field struct {
	Name     string
	Label    string
	Type     string
	Required bool
	Options  []string
	Dynamic  bool
}

// ValidateCommandInput rejects unknown keys first, then validates every supplied value,
// and only then checks required fields. The returned map contains normalized typed values;
// input is never modified in place.
func ValidateCommandInput(command string, fields []Field, input map[string]any) (map[string]any, error) {
	if input == nil {
		input = map[string]any{}
	}
	known := make(map[string]Field, len(fields))
	for _, field := range fields {
		if strings.TrimSpace(field.Name) == "" {
			return nil, fmt.Errorf("command %q has an invalid field schema", command)
		}
		known[field.Name] = field
	}
	unknown := make([]string, 0)
	for name := range input {
		if _, ok := known[name]; !ok {
			unknown = append(unknown, name)
		}
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		return nil, fmt.Errorf("command %q does not accept field(s): %s", command, strings.Join(unknown, ", "))
	}

	normalized := make(map[string]any, len(input))
	missing := make([]string, 0)
	for _, field := range fields {
		value, present := input[field.Name]
		label := strings.TrimSpace(field.Label)
		if label == "" {
			label = field.Name
		}
		if !present || value == nil {
			if field.Required {
				missing = append(missing, label)
			}
			continue
		}
		converted, empty, err := validateValue(field, value)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", label, err)
		}
		if empty {
			if field.Required {
				missing = append(missing, label)
			}
			continue
		}
		normalized[field.Name] = converted
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("required field(s) missing or empty: %s", strings.Join(missing, ", "))
	}
	return normalized, nil
}

func validateValue(field Field, value any) (any, bool, error) {
	switch strings.ToLower(strings.TrimSpace(field.Type)) {
	case "text", "textarea", "select", "date", "datetime", "color":
		text, ok := value.(string)
		if !ok {
			return nil, false, fmt.Errorf("must be a string")
		}
		text = strings.TrimSpace(text)
		if text == "" {
			return text, true, nil
		}
		if !utf8.ValidString(text) || strings.ContainsRune(text, '\x00') {
			return nil, false, fmt.Errorf("contains invalid text")
		}
		max := 1000
		switch field.Type {
		case "textarea":
			max = 50000
		case "select":
			max = 256
		case "date":
			max = 10
		case "datetime":
			max = 40
		case "color":
			max = 16
		}
		if utf8.RuneCountInString(text) > max {
			return nil, false, fmt.Errorf("must be at most %d characters", max)
		}
		switch field.Type {
		case "select":
			if !field.Dynamic {
				if len(field.Options) == 0 {
					return nil, false, fmt.Errorf("has no configured choices")
				}
				for _, option := range field.Options {
					if text == option {
						return text, false, nil
					}
				}
				return nil, false, fmt.Errorf("must be one of: %s", strings.Join(field.Options, ", "))
			}
		case "date":
			parsed, err := time.Parse("2006-01-02", text)
			if err != nil || parsed.Format("2006-01-02") != text {
				return nil, false, fmt.Errorf("must use YYYY-MM-DD")
			}
		case "datetime":
			valid := false
			for _, layout := range []string{"2006-01-02T15:04", "2006-01-02 15:04", time.RFC3339, time.RFC3339Nano} {
				if _, err := time.Parse(layout, text); err == nil {
					valid = true
					break
				}
			}
			if !valid {
				return nil, false, fmt.Errorf("must use YYYY-MM-DDTHH:MM or RFC3339")
			}
		case "color":
			if !validHexColor(text) {
				return nil, false, fmt.Errorf("must be a hex color such as #64748b")
			}
		}
		return text, false, nil

	case "number":
		if text, ok := value.(string); ok && strings.TrimSpace(text) == "" {
			return nil, true, nil
		}
		var number float64
		switch n := value.(type) {
		case int:
			number = float64(n)
		case int8:
			number = float64(n)
		case int16:
			number = float64(n)
		case int32:
			number = float64(n)
		case int64:
			number = float64(n)
		case uint:
			number = float64(n)
		case uint8:
			number = float64(n)
		case uint16:
			number = float64(n)
		case uint32:
			number = float64(n)
		case uint64:
			number = float64(n)
		case string:
			parsed, err := strconv.Atoi(strings.TrimSpace(n))
			if err != nil {
				return nil, false, fmt.Errorf("must be a whole number")
			}
			number = float64(parsed)
		case float32:
			number = float64(n)
		case float64:
			number = n
		default:
			return nil, false, fmt.Errorf("must be a number")
		}
		if math.IsNaN(number) || math.IsInf(number, 0) || math.Trunc(number) != number {
			return nil, false, fmt.Errorf("must be a finite whole number")
		}
		if number > float64(int(^uint(0)>>1)) || number < -float64(int(^uint(0)>>1))-1 {
			return nil, false, fmt.Errorf("is outside the supported integer range")
		}
		return int(number), false, nil

	case "boolean":
		switch b := value.(type) {
		case bool:
			return b, false, nil
		case string:
			switch strings.ToLower(strings.TrimSpace(b)) {
			case "true", "yes", "1":
				return true, false, nil
			case "false", "no", "0", "":
				return false, b == "", nil
			default:
				return nil, false, fmt.Errorf("must be true or false")
			}
		default:
			return nil, false, fmt.Errorf("must be a boolean")
		}
	default:
		return nil, false, fmt.Errorf("uses unsupported field type %q", field.Type)
	}
}

func validHexColor(value string) bool {
	if !strings.HasPrefix(value, "#") {
		return false
	}
	digits := value[1:]
	if len(digits) != 3 && len(digits) != 6 {
		return false
	}
	for _, char := range digits {
		if !(char >= '0' && char <= '9' || char >= 'a' && char <= 'f' || char >= 'A' && char <= 'F') {
			return false
		}
	}
	return true
}
