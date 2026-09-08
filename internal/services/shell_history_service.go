package services

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/vsuhanov/scripto/internal/storage"
)

const (
	ShellHistorySourceShell   = "shell"
	ShellHistorySourceScripto = "scripto"
)

type ShellHistoryRecord struct {
	ID               string
	Command          string
	WorkingDirectory string
	StartedAt        int64
	FinishedAt       int64
	ExitCode         *int
	SessionID        string
	Source           string
	SavedScriptID    string
}

func (r ShellHistoryRecord) Duration() time.Duration {
	if r.StartedAt == 0 || r.FinishedAt < r.StartedAt {
		return 0
	}
	return time.Duration(r.FinishedAt-r.StartedAt) * time.Second
}

type ShellHistoryQuery struct {
	// Filter is a Go regexp matched against the command text. Callers are
	// responsible for prefixing "(?i)" if they want a case-insensitive match,
	// and for validating the pattern compiles before querying.
	Filter           string
	WorkingDirectory string
	FailuresOnly     bool
	ExcludeSources   []string
	Limit            int
	Offset           int
}

type ShellHistoryService struct {
	db *sql.DB
}

func NewShellHistoryService() (*ShellHistoryService, error) {
	db, err := storage.OpenSQLite()
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite: %w", err)
	}
	return &ShellHistoryService{db: db}, nil
}

func NewShellHistoryServiceWithDB(db *sql.DB) *ShellHistoryService {
	return &ShellHistoryService{db: db}
}

func (s *ShellHistoryService) Close() {
	if s.db != nil {
		s.db.Close()
	}
}

func (s *ShellHistoryService) RecordStart(id, command, workingDir, sessionID, source string, startedAt int64) error {
	if source == "" {
		source = ShellHistorySourceShell
	}
	_, err := s.db.Exec(
		`INSERT INTO shell_history (id, command, working_directory, started_at, session_id, source)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
		     command = excluded.command,
		     working_directory = excluded.working_directory,
		     started_at = excluded.started_at,
		     session_id = excluded.session_id,
		     source = excluded.source`,
		id, command, workingDir, startedAt, sessionID, source,
	)
	if err != nil {
		return fmt.Errorf("failed to record command start: %w", err)
	}
	return nil
}

func (s *ShellHistoryService) RecordFinish(id string, exitCode int, finishedAt int64) error {
	_, err := s.db.Exec(
		`INSERT INTO shell_history (id, finished_at, exit_code)
		 VALUES (?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
		     finished_at = excluded.finished_at,
		     exit_code = excluded.exit_code`,
		id, finishedAt, exitCode,
	)
	if err != nil {
		return fmt.Errorf("failed to record command finish: %w", err)
	}
	return nil
}

func (s *ShellHistoryService) GetHistory(query ShellHistoryQuery) ([]ShellHistoryRecord, error) {
	var conditions []string
	var args []interface{}

	conditions = append(conditions, "command != ''")

	if query.Filter != "" {
		conditions = append(conditions, "command REGEXP ?")
		args = append(args, query.Filter)
	}
	if query.WorkingDirectory != "" {
		conditions = append(conditions, "working_directory = ?")
		args = append(args, query.WorkingDirectory)
	}
	if query.FailuresOnly {
		conditions = append(conditions, "exit_code IS NOT NULL AND exit_code != 0")
	}
	if len(query.ExcludeSources) > 0 {
		placeholders := make([]string, len(query.ExcludeSources))
		for i, source := range query.ExcludeSources {
			placeholders[i] = "?"
			args = append(args, source)
		}
		conditions = append(conditions, fmt.Sprintf("source NOT IN (%s)", strings.Join(placeholders, ", ")))
	}

	limit := query.Limit
	if limit <= 0 {
		limit = 100
	}
	args = append(args, limit, query.Offset)

	sqlQuery := fmt.Sprintf(
		`SELECT id, command, working_directory, started_at, finished_at, exit_code, session_id, source, saved_script_id
		 FROM shell_history
		 WHERE %s
		 ORDER BY started_at DESC, rowid DESC
		 LIMIT ? OFFSET ?`,
		strings.Join(conditions, " AND "),
	)

	rows, err := s.db.Query(sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query shell history: %w", err)
	}
	defer rows.Close()

	var records []ShellHistoryRecord
	for rows.Next() {
		var r ShellHistoryRecord
		var exitCode sql.NullInt64
		if err := rows.Scan(&r.ID, &r.Command, &r.WorkingDirectory, &r.StartedAt, &r.FinishedAt,
			&exitCode, &r.SessionID, &r.Source, &r.SavedScriptID); err != nil {
			return nil, fmt.Errorf("failed to scan shell history row: %w", err)
		}
		if exitCode.Valid {
			code := int(exitCode.Int64)
			r.ExitCode = &code
		}
		records = append(records, r)
	}
	return records, rows.Err()
}

func (s *ShellHistoryService) MarkSaved(id, scriptID string) error {
	_, err := s.db.Exec("UPDATE shell_history SET saved_script_id = ? WHERE id = ?", scriptID, id)
	if err != nil {
		return fmt.Errorf("failed to mark shell history entry as saved: %w", err)
	}
	return nil
}

func (s *ShellHistoryService) Delete(id string) error {
	_, err := s.db.Exec("DELETE FROM shell_history WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete shell history entry: %w", err)
	}
	return nil
}

func (s *ShellHistoryService) Prune(olderThanDays int) error {
	if olderThanDays <= 0 {
		return nil
	}
	cutoff := time.Now().AddDate(0, 0, -olderThanDays).Unix()
	res, err := s.db.Exec("DELETE FROM shell_history WHERE started_at > 0 AND started_at < ? AND saved_script_id = ''", cutoff)
	if err != nil {
		return fmt.Errorf("failed to prune shell history: %w", err)
	}
	if affected, err := res.RowsAffected(); err == nil && affected > 0 {
		log.Printf("ShellHistoryService: pruned %d entries older than %d days", affected, olderThanDays)
	}
	return nil
}
