package sqlite

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"idios/internal/domain"

	sqlite3 "github.com/mattn/go-sqlite3"
)

// Value conversion, timestamp formatting, and driver-to-domain error mapping live here.

func nowText() string             { return domain.Now().Format(time.RFC3339Nano) }
func timeText(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }

func parseTime(v any) time.Time {
	if x, ok := v.(string); ok && x != "" {
		if t, err := time.Parse(time.RFC3339Nano, x); err == nil {
			return t
		}
		if t, err := time.Parse(time.RFC3339, x); err == nil {
			return t
		}
	}
	return time.Time{}
}

func str(r Row, k string) string {
	if r == nil || r[k] == nil {
		return ""
	}
	return fmt.Sprint(r[k])
}

func i64(v any) int64 {
	switch x := v.(type) {
	case int64:
		return x
	case int:
		return int64(x)
	case float64:
		return int64(x)
	case string:
		var n int64
		fmt.Sscan(x, &n)
		return n
	}
	return 0
}

func boolRow(r Row, k string) bool { return i64(r[k]) != 0 }

func ptrString(v any) *string {
	if v == nil {
		return nil
	}
	s := fmt.Sprint(v)
	if s == "" {
		return nil
	}
	return &s
}

func ptrTime(v any) *time.Time {
	if v == nil {
		return nil
	}
	t := parseTime(v)
	if t.IsZero() {
		return nil
	}
	return &t
}

// classify maps typed go-sqlite3 constraint codes to domain errors. Avoid matching error
// strings: SQLite exposes stable primary and extended result codes for this purpose.
func classify(err error) error {
	if err == nil {
		return nil
	}
	var sqliteErr sqlite3.Error
	if !errors.As(err, &sqliteErr) {
		return err
	}
	switch sqliteErr.ExtendedCode {
	case sqlite3.ErrConstraintUnique, sqlite3.ErrConstraintPrimaryKey:
		return fmt.Errorf("%w: %v", domain.ErrConflict, err)
	case sqlite3.ErrConstraintForeignKey:
		return fmt.Errorf("%w: referenced item does not exist or is still in use", domain.ErrInvalid)
	case sqlite3.ErrConstraintCheck, sqlite3.ErrConstraintNotNull:
		return fmt.Errorf("%w: %v", domain.ErrInvalid, err)
	default:
		if sqliteErr.Code == sqlite3.ErrConstraint {
			return fmt.Errorf("%w: %v", domain.ErrInvalid, err)
		}
		return err
	}
}

// marshalPayload is the JSON blob stored on an activity event; an unencodable value
// degrades to an empty string rather than failing the write.
func marshalPayload(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(b)
}
