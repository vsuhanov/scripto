package storage

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"sort"
	"strings"
)

//go:embed migrations/001_initial.sql
var migration001 string

//go:embed migrations/002_shell_history.sql
var migration002 string

var migrations = []struct {
	name string
	sql  string
}{
	{"001_initial", migration001},
	{"002_shell_history", migration002},
}

const migrationLockAttempts = 5

func sortedMigrations() []struct{ name, sql string } {
	sorted := make([]struct{ name, sql string }, len(migrations))
	copy(sorted, migrations)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].name < sorted[j].name
	})
	return sorted
}

func appliedMigrations(q interface {
	Query(string, ...interface{}) (*sql.Rows, error)
}) (map[string]bool, error) {
	rows, err := q.Query("SELECT name FROM schema_migrations")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	applied := make(map[string]bool)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		applied[name] = true
	}
	return applied, rows.Err()
}

func hasPendingMigrations(db *sql.DB) bool {
	applied, err := appliedMigrations(db)
	if err != nil {
		return true
	}
	for _, m := range migrations {
		if !applied[m.name] {
			return true
		}
	}
	return false
}

func applyMigrations(db *sql.DB) error {
	if !hasPendingMigrations(db) {
		return nil
	}

	var lastErr error
	for attempt := 0; attempt < migrationLockAttempts; attempt++ {
		lastErr = applyMigrationsLocked(db)
		if lastErr == nil {
			return nil
		}
		if !hasPendingMigrations(db) {
			return nil
		}
	}
	return lastErr
}

func applyMigrationsLocked(db *sql.DB) error {
	ctx := context.Background()
	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("failed to acquire connection: %w", err)
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return fmt.Errorf("failed to acquire migration lock: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_, _ = conn.ExecContext(ctx, "ROLLBACK")
		}
	}()

	if _, err := conn.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		name TEXT PRIMARY KEY,
		applied_at INTEGER NOT NULL
	)`); err != nil {
		return fmt.Errorf("failed to create schema_migrations table: %w", err)
	}

	rows, err := conn.QueryContext(ctx, "SELECT name FROM schema_migrations")
	if err != nil {
		return fmt.Errorf("failed to query applied migrations: %w", err)
	}
	applied := make(map[string]bool)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			return fmt.Errorf("failed to scan migration name: %w", err)
		}
		applied[name] = true
	}
	rows.Close()

	for _, m := range sortedMigrations() {
		if applied[m.name] {
			continue
		}
		for _, stmt := range strings.Split(m.sql, ";") {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" {
				continue
			}
			if _, err := conn.ExecContext(ctx, stmt); err != nil {
				return fmt.Errorf("failed to apply migration %s: %w", m.name, err)
			}
		}
		if _, err := conn.ExecContext(ctx,
			"INSERT OR IGNORE INTO schema_migrations (name, applied_at) VALUES (?, strftime('%s', 'now'))", m.name); err != nil {
			return fmt.Errorf("failed to record migration %s: %w", m.name, err)
		}
	}

	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return fmt.Errorf("failed to commit migrations: %w", err)
	}
	committed = true
	return nil
}
