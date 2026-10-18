// Package sqlite implements ports.Store using SQLite through database/sql and mattn/go-sqlite3.
// SQL is grouped by domain area in separate files; this file owns the handle lifecycle only.
package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"idios/internal/storage/sqlite/migrations"
)

// Store is the durable SQLite adapter. It is safe for concurrent use; database/sql owns the
// connection pool and each multi-step mutation uses a transaction through run.
type Store struct {
	db   *sql.DB
	path string
}

// OpenStore opens (creating parent directories if necessary) and upgrades the Engine database.
// The database is closed automatically if migration fails.
func OpenStore(path string) (*Store, error) {
	db, err := openDatabase(path, false)
	if err != nil {
		return nil, err
	}
	if err := migrations.Apply(context.Background(), db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("initialize IDIOS database: %w", err)
	}
	return &Store{db: db, path: path}, nil
}

// Close releases the connection pool. Call it when the Engine process exits.
func (s *Store) Close() error { return s.db.Close() }

// Ping verifies that the connection pool can reach SQLite.
func (s *Store) Ping(ctx context.Context) error {
	if err := s.db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping SQLite database: %w", err)
	}
	return nil
}
