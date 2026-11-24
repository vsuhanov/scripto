package services

import (
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/vsuhanov/scripto/entities"
	"github.com/vsuhanov/scripto/internal/storage"
)

type ScriptStats struct {
	LastExecutionTime time.Time
	ExecutionCount    int
	Frecency          float64
}

const frecencyHalfLifeHours = 168.0

const executionRecordColumns = `id, execution_timestamp, script_id, executed_script, original_script, placeholder_values, working_directory, script_object_definition, executed_script_hash, original_script_hash, invoked_from, exit_code, finished_at`

type ExecutionRecord struct {
	ID                     string
	ExecutionTimestamp     int64
	ScriptID               string
	ExecutedScript         string
	OriginalScript         string
	PlaceholderValues      map[string]string
	WorkingDirectory       string
	ScriptObjectDefinition string
	ExecutedScriptHash     string
	OriginalScriptHash     string
	ScriptName             string
	ScriptScope            string
	InvokedFrom            string
	ExitCode               *int
	FinishedAt             int64
}

type ExecutionHistoryService struct {
	db *sql.DB
}

func NewExecutionHistoryService() (*ExecutionHistoryService, error) {
	dbPath, _ := storage.GetSQLitePath()
	log.Printf("NewExecutionHistoryService: opening sqlite at %q", dbPath)
	db, err := storage.OpenSQLite()
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite: %w", err)
	}
	log.Printf("NewExecutionHistoryService: sqlite opened successfully")
	return &ExecutionHistoryService{db: db}, nil
}

func (s *ExecutionHistoryService) DB() *sql.DB {
	return s.db
}

func (s *ExecutionHistoryService) Close() {
	if s.db != nil {
		s.db.Close()
	}
}

func sha256hex(data string) string {
	h := sha256.Sum256([]byte(data))
	return fmt.Sprintf("%x", h)
}

func (s *ExecutionHistoryService) SaveExecution(record ExecutionRecord) {
	if record.ID == "" {
		record.ID = uuid.New().String()
	}
	if record.ExecutionTimestamp == 0 {
		record.ExecutionTimestamp = time.Now().Unix()
	}
	record.ExecutedScriptHash = sha256hex(record.ExecutedScript)
	record.OriginalScriptHash = sha256hex(record.OriginalScript)

	pvJSON, err := json.Marshal(record.PlaceholderValues)
	if err != nil {
		pvJSON = []byte("{}")
	}

	_, err = s.db.Exec(
		`INSERT INTO execution_history (id, execution_timestamp, script_id, executed_script, original_script, placeholder_values, working_directory, script_object_definition, executed_script_hash, original_script_hash, invoked_from)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		record.ID,
		record.ExecutionTimestamp,
		record.ScriptID,
		record.ExecutedScript,
		record.OriginalScript,
		string(pvJSON),
		record.WorkingDirectory,
		record.ScriptObjectDefinition,
		record.ExecutedScriptHash,
		record.OriginalScriptHash,
		record.InvokedFrom,
	)
	if err != nil {
		log.Printf("SaveExecution: failed to insert id=%q script_id=%q: %v", record.ID, record.ScriptID, err)
	}
}

func (s *ExecutionHistoryService) RecordResult(id string, exitCode int, finishedAt int64) error {
	_, err := s.db.Exec("UPDATE execution_history SET exit_code = ?, finished_at = ? WHERE id = ?", exitCode, finishedAt, id)
	if err != nil {
		return fmt.Errorf("failed to record execution result: %w", err)
	}
	return nil
}

func (s *ExecutionHistoryService) GetFrecencyScores() map[string]float64 {
	stats, err := s.GetAllScriptStats()
	scores := make(map[string]float64, len(stats))
	if err != nil {
		return scores
	}
	for id, stat := range stats {
		scores[id] = stat.Frecency
	}
	return scores
}

func (s *ExecutionHistoryService) GetAllScriptStats() (map[string]ScriptStats, error) {
	rows, err := s.db.Query(
		`SELECT script_id, MAX(execution_timestamp), COUNT(*),
		        SUM(1.0 / (1.0 + ((? - execution_timestamp) / 3600.0) / ?))
		 FROM execution_history GROUP BY script_id`,
		time.Now().Unix(), frecencyHalfLifeHours,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats := make(map[string]ScriptStats)
	for rows.Next() {
		var scriptID string
		var lastTs int64
		var count int
		var frecency float64
		if err := rows.Scan(&scriptID, &lastTs, &count, &frecency); err != nil {
			return nil, err
		}
		stats[scriptID] = ScriptStats{
			LastExecutionTime: time.Unix(lastTs, 0),
			ExecutionCount:    count,
			Frecency:          frecency,
		}
	}
	return stats, rows.Err()
}

func (s *ExecutionHistoryService) GetHistory(filter string, limit, offset int) ([]ExecutionRecord, error) {
	var rows *sql.Rows
	var err error
	if filter != "" {
		rows, err = s.db.Query(
			`SELECT `+executionRecordColumns+`
			 FROM execution_history
			 WHERE executed_script LIKE ? OR script_id = ?
			 ORDER BY execution_timestamp DESC LIMIT ? OFFSET ?`,
			"%"+filter+"%", filter, limit, offset,
		)
	} else {
		rows, err = s.db.Query(
			`SELECT `+executionRecordColumns+`
			 FROM execution_history
			 ORDER BY execution_timestamp DESC LIMIT ? OFFSET ?`,
			limit, offset,
		)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanExecutionRecords(rows)
}

func (s *ExecutionHistoryService) GetScriptIDsRunFromDirectory(dir string) ([]string, error) {
	rows, err := s.db.Query(
		`SELECT DISTINCT script_id FROM execution_history WHERE invoked_from = ?`,
		dir,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (s *ExecutionHistoryService) GetScriptHistory(scriptID string, limit int) ([]ExecutionRecord, error) {
	rows, err := s.db.Query(
		`SELECT `+executionRecordColumns+`
		 FROM execution_history
		 WHERE script_id = ?
		 ORDER BY execution_timestamp DESC LIMIT ?`,
		scriptID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanExecutionRecords(rows)
}

func scanExecutionRecords(rows *sql.Rows) ([]ExecutionRecord, error) {
	var records []ExecutionRecord
	for rows.Next() {
		var r ExecutionRecord
		var pvJSON string
		var exitCode sql.NullInt64
		if err := rows.Scan(
			&r.ID, &r.ExecutionTimestamp, &r.ScriptID, &r.ExecutedScript,
			&r.OriginalScript, &pvJSON, &r.WorkingDirectory,
			&r.ScriptObjectDefinition, &r.ExecutedScriptHash, &r.OriginalScriptHash,
			&r.InvokedFrom, &exitCode, &r.FinishedAt,
		); err != nil {
			return nil, err
		}
		if exitCode.Valid {
			code := int(exitCode.Int64)
			r.ExitCode = &code
		}
		if err := json.Unmarshal([]byte(pvJSON), &r.PlaceholderValues); err != nil {
			r.PlaceholderValues = map[string]string{}
		}
		var scriptObj struct {
			Name  string `json:"name"`
			Scope string `json:"scope"`
		}
		if err := json.Unmarshal([]byte(r.ScriptObjectDefinition), &scriptObj); err == nil {
			r.ScriptName = scriptObj.Name
			r.ScriptScope = scriptObj.Scope
		}
		records = append(records, r)
	}
	return records, rows.Err()
}

func BuildExecutionRecord(script *entities.Script, executedScript, originalScript string, placeholderValues map[string]string, workingDir, invokedFrom string) ExecutionRecord {
	scriptJSON, _ := json.Marshal(script)
	return ExecutionRecord{
		ID:                     uuid.New().String(),
		InvokedFrom:            invokedFrom,
		ScriptID:               script.ID,
		ExecutedScript:         executedScript,
		OriginalScript:         originalScript,
		PlaceholderValues:      placeholderValues,
		WorkingDirectory:       workingDir,
		ScriptObjectDefinition: string(scriptJSON),
	}
}
