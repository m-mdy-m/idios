// Connection settings for the SQLite adapter. This file is the only place in the codebase
// allowed to call sql.Open or name a driver, so every connection the Engine uses shares one
// DSN and one set of pragmas.
package sqlite

import (
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	sqlite3 "github.com/mattn/go-sqlite3" // registers the "sqlite3" driver
)

// memoryPath is SQLite's in-memory database name. It is not a filesystem path, so it skips
// the URI escaping and the directory creation below.
const memoryPath = ":memory:"

// openAttempts covers the startup race when two processes create the same fresh database at
// once: SQLite reports SQLITE_BUSY at once while another connection is switching the journal
// mode, so opening retries briefly instead of failing the whole command.
const (
	openAttempts = 5
	openBackoff  = 20 * time.Millisecond
)

// writableOptions apply to the Engine database. WAL keeps readers from blocking on the
// writer; IMMEDIATE takes the write lock when a transaction starts, so a second writer fails
// fast instead of halfway through; foreign keys back the schema's guarantees; the busy
// timeout covers the brief overlap when another process writes.
var writableOptions = url.Values{
	"_busy_timeout": {"5000"},
	"_foreign_keys": {"on"},
	"_journal_mode": {"WAL"},
	"_synchronous":  {"NORMAL"},
	"_txlock":       {"immediate"},
}

// readOnlyOptions is what a legacy import may use: no journal, no transaction mode, nothing
// that would write. SQLite applies mode=ro itself because the DSN is a file URI.
var readOnlyOptions = url.Values{
	"_foreign_keys": {"on"},
	"mode":          {"ro"},
}

// dataSourceName builds the driver DSN for a database path. The path is escaped into a file
// URI so spaces, '#' and '?' in a filename cannot change how SQLite reads the query string.
func dataSourceName(path string, readOnly bool) (string, error) {
	options := writableOptions
	if readOnly {
		options = readOnlyOptions
	}
	if path == memoryPath {
		if readOnly {
			return "", fmt.Errorf("an in-memory database cannot be opened read-only")
		}
		return memoryPath + "?" + options.Encode(), nil
	}

	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve SQLite path %q: %w", path, err)
	}
	uriPath := filepath.ToSlash(absolute)
	if !strings.HasPrefix(uriPath, "/") {
		uriPath = "/" + uriPath // Windows drive letters: C:/db becomes /C:/db
	}
	dsn := url.URL{Scheme: "file", Path: uriPath, RawQuery: options.Encode()}
	return dsn.String(), nil
}

// openDatabase opens one SQLite database handle, creating parent directories for a writable
// database. It pings so an unusable path fails at startup instead of at the first command.
func openDatabase(path string, readOnly bool) (*sql.DB, error) {
	dsn, err := dataSourceName(path, readOnly)
	if err != nil {
		return nil, err
	}
	if !readOnly && path != memoryPath {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, fmt.Errorf("create directory for SQLite database %q: %w", path, err)
		}
	}
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("open SQLite database: %w", err)
	}
	if path == memoryPath {
		// Every pooled connection to ":memory:" would get its own empty database.
		db.SetMaxOpenConns(1)
	}
	for attempt := 0; ; attempt++ {
		err = db.Ping()
		if err == nil {
			return db, nil
		}
		if attempt == openAttempts-1 || !contended(err) {
			_ = db.Close()
			return nil, fmt.Errorf("open SQLite database: %w", err)
		}
		time.Sleep(openBackoff)
	}
}

// contended reports SQLite's "another connection holds the lock" result codes.
func contended(err error) bool {
	var sqliteErr sqlite3.Error
	if !errors.As(err, &sqliteErr) {
		return false
	}
	return sqliteErr.Code == sqlite3.ErrBusy || sqliteErr.Code == sqlite3.ErrLocked
}
