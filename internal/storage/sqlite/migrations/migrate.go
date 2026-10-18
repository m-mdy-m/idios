// Package migrations owns the SQLite schema history. Each migration is an ordered SQL file
// named NNNN_description.sql and is applied exactly once in its own transaction.
package migrations

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"
)

//go:embed *.sql
var files embed.FS

type migration struct {
	version int
	name    string
	sql     string
}

// Apply upgrades db to the newest embedded schema. It refuses a database created by a newer
// IDIOS build rather than silently changing it or attempting a downgrade.
func Apply(ctx context.Context, db *sql.DB) error {
	all, err := load()
	if err != nil {
		return err
	}
	if len(all) == 0 {
		return fmt.Errorf("no SQLite migrations are embedded")
	}

	var current int
	if err := db.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&current); err != nil {
		return fmt.Errorf("read SQLite schema version: %w", err)
	}
	latest := all[len(all)-1].version
	if current > latest {
		return fmt.Errorf("database schema version %d is newer than this IDIOS Engine (supports %d); use a compatible Engine version", current, latest)
	}

	for _, item := range all {
		if item.version <= current {
			continue
		}
		if item.version != current+1 {
			return fmt.Errorf("SQLite migration sequence has a gap: database is at %d but next migration is %d (%s)", current, item.version, item.name)
		}
		if err := applyOne(ctx, db, item, latest); err != nil {
			return err
		}
		current = item.version
	}
	return nil
}

func load() ([]migration, error) {
	entries, err := fs.ReadDir(files, ".")
	if err != nil {
		return nil, fmt.Errorf("list embedded SQLite migrations: %w", err)
	}

	result := make([]migration, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		name := entry.Name()
		prefix, _, ok := strings.Cut(name, "_")
		if !ok || len(prefix) != 4 {
			return nil, fmt.Errorf("invalid migration filename %q: expected NNNN_description.sql", name)
		}
		version, err := strconv.Atoi(prefix)
		if err != nil || version < 1 {
			return nil, fmt.Errorf("invalid migration version in filename %q", name)
		}
		contents, err := files.ReadFile(name)
		if err != nil {
			return nil, fmt.Errorf("read embedded migration %q: %w", name, err)
		}
		result = append(result, migration{version: version, name: name, sql: string(contents)})
	}

	sort.Slice(result, func(i, j int) bool { return result[i].version < result[j].version })
	for i := range result {
		if i > 0 && result[i-1].version == result[i].version {
			return nil, fmt.Errorf("duplicate SQLite migration version %d (%s and %s)", result[i].version, result[i-1].name, result[i].name)
		}
		expected := i + 1
		if result[i].version != expected {
			return nil, fmt.Errorf("SQLite migration sequence must be contiguous from version 1; found version %d where %d was expected (%s)", result[i].version, expected, result[i].name)
		}
	}
	return result, nil
}

func applyOne(ctx context.Context, db *sql.DB, item migration, latest int) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin SQLite migration %s: %w", item.name, err)
	}
	defer func() { _ = tx.Rollback() }()

	// Another Engine process may have applied this migration after Apply read the initial
	// user_version. The write transaction uses BEGIN IMMEDIATE, so this re-check serializes
	// startup migrations and prevents two processes from creating the same schema concurrently.
	var current int
	if err := tx.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&current); err != nil {
		return fmt.Errorf("read schema version inside migration %s: %w", item.name, err)
	}
	if current > latest {
		return fmt.Errorf("database schema version %d is newer than this IDIOS Engine (supports %d); use a compatible Engine version", current, latest)
	}
	if current >= item.version {
		return nil
	}
	if current != item.version-1 {
		return fmt.Errorf("cannot apply SQLite migration %s: database is at version %d", item.name, current)
	}

	// go-sqlite3 supports executing a SQL script when no bind parameters are supplied.
	if _, err := tx.ExecContext(ctx, item.sql); err != nil {
		return fmt.Errorf("apply SQLite migration %s: %w", item.name, err)
	}
	if _, err := tx.ExecContext(ctx, fmt.Sprintf(`PRAGMA user_version = %d`, item.version)); err != nil {
		return fmt.Errorf("set SQLite schema version to %d: %w", item.version, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit SQLite migration %s: %w", item.name, err)
	}
	return nil
}
