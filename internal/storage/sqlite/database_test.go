package sqlite

import (
	"context"
	"errors"
	"net/url"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	sqlite3 "github.com/mattn/go-sqlite3"
	"idios/internal/domain"
)

func TestDataSourceNameEscapesFilePaths(t *testing.T) {
	path := filepath.Join(t.TempDir(), "db # name?.sqlite3")
	dsn, err := dataSourceName(path, false)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parse generated DSN %q: %v", dsn, err)
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	decodedPath := filepath.FromSlash(parsed.Path)
	if runtime.GOOS == "windows" && strings.HasPrefix(decodedPath, `\`) && len(decodedPath) > 2 && decodedPath[2] == ':' {
		decodedPath = decodedPath[1:]
	}
	if filepath.Clean(decodedPath) != filepath.Clean(absolute) {
		t.Fatalf("DSN path = %q, want %q", parsed.Path, absolute)
	}
	query := parsed.Query()
	for key, want := range map[string]string{
		"_busy_timeout": "5000",
		"_foreign_keys": "on",
		"_journal_mode": "WAL",
		"_synchronous":  "NORMAL",
		"_txlock":       "immediate",
	} {
		if got := query.Get(key); got != want {
			t.Errorf("DSN %s = %q, want %q", key, got, want)
		}
	}
}

// reservedFileName is a filename that needs escaping in the DSN. '?' is legal in a POSIX name
// but rejected by Windows, so the on-disk probe drops it there; the DSN probe above keeps it.
func reservedFileName() string {
	if runtime.GOOS == "windows" {
		return "idios # data.sqlite3"
	}
	return "idios # data?.sqlite3"
}

func TestOpenStoreSupportsReservedCharactersInPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), reservedFileName())
	store, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Ping(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestReadOnlyDSNDoesNotEnableWritePragmas(t *testing.T) {
	dsn, err := dataSourceName(filepath.Join(t.TempDir(), "legacy.sqlite3"), true)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	query := parsed.Query()
	if query.Get("mode") != "ro" {
		t.Fatalf("read-only DSN mode = %q, want ro", query.Get("mode"))
	}
	if query.Has("_journal_mode") || query.Has("_synchronous") || query.Has("_txlock") {
		t.Fatalf("read-only DSN contains writable options: %v", query)
	}
	if query.Get("_foreign_keys") != "on" {
		t.Fatalf("foreign keys are not enabled for read-only handle: %v", query)
	}
}

func TestOpenStoreConfiguresSQLiteAndAppliesMigrations(t *testing.T) {
	ctx := context.Background()
	store, err := OpenStore(filepath.Join(t.TempDir(), "nested", "engine.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	if err := store.Ping(ctx); err != nil {
		t.Fatal(err)
	}
	for pragma, want := range map[string]string{
		"foreign_keys": "1",
		"journal_mode": "wal",
	} {
		row, ok, err := store.p().row(ctx, "PRAGMA "+pragma)
		if err != nil || !ok {
			t.Fatalf("read pragma %s: ok=%v err=%v", pragma, ok, err)
		}
		got := str(row, pragma)
		if got != want {
			t.Errorf("PRAGMA %s = %q, want %q (row=%v)", pragma, got, want, row)
		}
	}
	row, ok, err := store.p().row(ctx, `PRAGMA user_version`)
	if err != nil || !ok || i64(row["user_version"]) != 17 {
		t.Fatalf("schema version after migration = %v, ok=%v err=%v; want 17", row, ok, err)
	}
}

func TestConstraintErrorsUseSQLiteResultCodes(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "engine.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ctx := context.Background()
	if _, err := store.db.ExecContext(ctx, `CREATE TABLE unique_probe (value TEXT UNIQUE)`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(ctx, `INSERT INTO unique_probe(value) VALUES('same')`); err != nil {
		t.Fatal(err)
	}
	_, err = store.db.ExecContext(ctx, `INSERT INTO unique_probe(value) VALUES('same')`)
	if err == nil {
		t.Fatal("expected unique constraint violation")
	}
	var sqliteErr sqlite3.Error
	if !errors.As(err, &sqliteErr) || sqliteErr.ExtendedCode != sqlite3.ErrConstraintUnique {
		t.Fatalf("driver error = %#v, want typed unique constraint error", err)
	}
	if classified := classify(err); !errors.Is(classified, domain.ErrConflict) {
		t.Fatalf("classified error = %v, want ErrConflict", classified)
	}
}

func TestConcurrentOpenStoreAppliesMigrationsOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shared-engine.sqlite3")
	const workers = 6
	start := make(chan struct{})
	errs := make(chan error, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			store, err := OpenStore(path)
			if err != nil {
				errs <- err
				return
			}
			errs <- store.Close()
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent OpenStore failed: %v", err)
		}
	}

	store, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	row, ok, err := store.p().row(context.Background(), `PRAGMA user_version`)
	if err != nil || !ok || i64(row["user_version"]) != 17 {
		t.Fatalf("schema version after concurrent open = %v, ok=%v err=%v; want 17", row, ok, err)
	}
}

func TestOpenStoreRefusesNewerSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "future.sqlite3")
	db, err := openDatabase(path, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(context.Background(), `PRAGMA user_version = 999`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenStore(path); err == nil {
		t.Fatal("expected a newer schema version to be refused")
	}
}

func TestTransactionRollsBackOnError(t *testing.T) {
	ctx := context.Background()
	store, err := OpenStore(filepath.Join(t.TempDir(), "engine.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err := store.db.ExecContext(ctx, `CREATE TABLE rollback_probe (value INTEGER)`); err != nil {
		t.Fatal(err)
	}
	boom := context.Canceled
	err = store.run(ctx, func(p provider) error {
		if _, err := p.changes(ctx, `INSERT INTO rollback_probe(value) VALUES(1)`); err != nil {
			return err
		}
		return boom
	})
	if err != boom {
		t.Fatalf("transaction error = %v, want %v", err, boom)
	}
	var count int
	if err := store.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM rollback_probe`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("rolled-back insert remains: count=%d", count)
	}
}
