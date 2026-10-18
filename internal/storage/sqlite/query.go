package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"idios/internal/domain"
)

// Row is a single SQL result addressed by column name. Keeping the mapping here avoids a
// parallel positional-column list in every entity adapter; all conversion helpers live in
// values.go and all query execution remains backed by database/sql.
type Row map[string]any

type queryer interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

type provider struct{ q queryer }

func (s *Store) p() provider { return provider{q: s.db} }

// run executes one unit of work in a write transaction. The DSN selects BEGIN IMMEDIATE so
// a writer acquires its lock at transaction start and can re-check invariants before mutation.
func (s *Store) run(ctx context.Context, fn func(provider) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return classify(err)
	}

	if err := fn(provider{q: tx}); err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			return errors.Join(err, fmt.Errorf("rollback SQLite transaction: %w", rollbackErr))
		}
		return err
	}
	if err := tx.Commit(); err != nil {
		return classify(err)
	}
	return nil
}

func (p provider) all(ctx context.Context, statement string, args ...any) ([]Row, error) {
	rows, err := p.q.QueryContext(ctx, statement, args...)
	if err != nil {
		return nil, classify(err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("read SQLite columns: %w", err)
	}

	result := make([]Row, 0)
	for rows.Next() {
		row, err := scanRow(rows, columns)
		if err != nil {
			return nil, fmt.Errorf("scan SQLite row: %w", err)
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, classify(err)
	}
	return result, nil
}

func (p provider) row(ctx context.Context, statement string, args ...any) (Row, bool, error) {
	rows, err := p.q.QueryContext(ctx, statement, args...)
	if err != nil {
		return nil, false, classify(err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, false, fmt.Errorf("read SQLite columns: %w", err)
	}
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, false, classify(err)
		}
		return nil, false, nil
	}
	row, err := scanRow(rows, columns)
	if err != nil {
		return nil, false, fmt.Errorf("scan SQLite row: %w", err)
	}
	return row, true, nil
}

// must loads one row or returns the domain-level not-found error with a useful descriptor.
func (p provider) must(ctx context.Context, descriptor, statement string, args ...any) (Row, error) {
	row, ok, err := p.row(ctx, statement, args...)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("%w: %s", domain.ErrNotFound, descriptor)
	}
	return row, nil
}

func (p provider) changes(ctx context.Context, statement string, args ...any) (int64, error) {
	result, err := p.q.ExecContext(ctx, statement, args...)
	if err != nil {
		return 0, classify(err)
	}
	changes, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("read SQLite affected-row count: %w", err)
	}
	return changes, nil
}

func (p provider) exists(ctx context.Context, statement string, args ...any) (bool, error) {
	_, ok, err := p.row(ctx, statement, args...)
	return ok, err
}

func scanRow(rows *sql.Rows, columns []string) (Row, error) {
	values := make([]any, len(columns))
	destinations := make([]any, len(columns))
	for i := range values {
		destinations[i] = &values[i]
	}
	if err := rows.Scan(destinations...); err != nil {
		return nil, err
	}
	row := make(Row, len(columns))
	for i, column := range columns {
		row[column] = normalizeValue(values[i])
	}
	return row, nil
}

func normalizeValue(value any) any {
	if bytes, ok := value.([]byte); ok {
		return string(bytes)
	}
	return value
}

func collect[T any](rows []Row, from func(Row) T) []T {
	result := make([]T, 0, len(rows))
	for _, row := range rows {
		result = append(result, from(row))
	}
	return result
}
