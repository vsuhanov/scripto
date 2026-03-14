package storage

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

func GetSQLitePath() (string, error) {
	if customPath := os.Getenv("SCRIPTO_SQLITE_DB_PATH"); customPath != "" {
		return customPath, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, configDir, "scripto.sqlite"), nil
}

func OpenSQLite() (*sql.DB, error) {
	dbPath, err := GetSQLitePath()
	if err != nil {
		return nil, fmt.Errorf("failed to get sqlite path: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create sqlite directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite: %w", err)
	}

	if err := applyMigrations(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to apply migrations: %w", err)
	}

	return db, nil
}
