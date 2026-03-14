package storage

import (
	"database/sql"
	_ "embed"
	"fmt"
	"sort"
	"strings"
)

//go:embed migrations/001_initial.sql
var migration001 string

var migrations = []struct {
	name string
	sql  string
}{
	{"001_initial", migration001},
}

func applyMigrations(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		name TEXT PRIMARY KEY,
		applied_at INTEGER NOT NULL
	)`)
	if err != nil {
		return fmt.Errorf("failed to create schema_migrations table: %w", err)
	}

	rows, err := db.Query("SELECT name FROM schema_migrations")
	if err != nil {
		return fmt.Errorf("failed to query applied migrations: %w", err)
	}
	defer rows.Close()

	applied := make(map[string]bool)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return fmt.Errorf("failed to scan migration name: %w", err)
		}
		applied[name] = true
	}

	sorted := make([]struct{ name, sql string }, len(migrations))
	copy(sorted, migrations)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].name < sorted[j].name
	})

	for _, m := range sorted {
		if applied[m.name] {
			continue
		}
		statements := strings.Split(m.sql, ";")
		for _, stmt := range statements {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" {
				continue
			}
			if _, err := db.Exec(stmt); err != nil {
				return fmt.Errorf("failed to apply migration %s: %w", m.name, err)
			}
		}
		if _, err := db.Exec("INSERT INTO schema_migrations (name, applied_at) VALUES (?, strftime('%s', 'now'))", m.name); err != nil {
			return fmt.Errorf("failed to record migration %s: %w", m.name, err)
		}
	}

	return nil
}
