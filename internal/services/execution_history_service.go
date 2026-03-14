package services

import (
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"scripto/entities"
	"scripto/internal/storage"
)

type ScriptStats struct {
	LastExecutionTime time.Time
	ExecutionCount    int
}

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
	log.Printf("SaveExecution: scriptID=%q executedScript=%q", record.ScriptID, record.ExecutedScript)
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

	log.Printf("SaveExecution: inserting row id=%q script_id=%q ts=%d", record.ID, record.ScriptID, record.ExecutionTimestamp)
	_, err = s.db.Exec(
		`INSERT INTO execution_history (id, execution_timestamp, script_id, executed_script, original_script, placeholder_values, working_directory, script_object_definition, executed_script_hash, original_script_hash)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
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
	)
	if err != nil {
		log.Printf("SaveExecution: failed to insert: %v", err)
	} else {
		log.Printf("SaveExecution: inserted successfully id=%q", record.ID)
	}
}

func (s *ExecutionHistoryService) GetLastExecutionTime(scriptID string) (time.Time, error) {
	var ts int64
	err := s.db.QueryRow(
		"SELECT execution_timestamp FROM execution_history WHERE script_id = ? ORDER BY execution_timestamp DESC LIMIT 1",
		scriptID,
	).Scan(&ts)
	if err == sql.ErrNoRows {
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, err
	}
	return time.Unix(ts, 0), nil
}

func (s *ExecutionHistoryService) GetExecutionCount(scriptID string) (int, error) {
	var count int
	err := s.db.QueryRow(
		"SELECT COUNT(*) FROM execution_history WHERE script_id = ?",
		scriptID,
	).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (s *ExecutionHistoryService) GetFrecencyScores() map[string]float64 {
	rows, err := s.db.Query(`SELECT script_id, execution_timestamp FROM execution_history`)
	if err != nil {
		return map[string]float64{}
	}
	defer rows.Close()

	const halfLife = 168.0
	now := time.Now()
	scores := map[string]float64{}
	for rows.Next() {
		var scriptID string
		var ts int64
		if err := rows.Scan(&scriptID, &ts); err != nil {
			continue
		}
		hoursSince := now.Sub(time.Unix(ts, 0)).Hours()
		scores[scriptID] += 1.0 / (1.0 + hoursSince/halfLife)
	}
	return scores
}

func (s *ExecutionHistoryService) GetAllScriptStats() (map[string]ScriptStats, error) {
	rows, err := s.db.Query(
		`SELECT script_id, MAX(execution_timestamp), COUNT(*) FROM execution_history GROUP BY script_id`,
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
		if err := rows.Scan(&scriptID, &lastTs, &count); err != nil {
			return nil, err
		}
		stats[scriptID] = ScriptStats{
			LastExecutionTime: time.Unix(lastTs, 0),
			ExecutionCount:    count,
		}
	}
	return stats, nil
}

func (s *ExecutionHistoryService) GetHistory(filter string, limit, offset int) ([]ExecutionRecord, error) {
	var rows *sql.Rows
	var err error
	if filter != "" {
		rows, err = s.db.Query(
			`SELECT id, execution_timestamp, script_id, executed_script, original_script, placeholder_values, working_directory, script_object_definition, executed_script_hash, original_script_hash
			 FROM execution_history
			 WHERE executed_script LIKE ? OR script_id = ?
			 ORDER BY execution_timestamp DESC LIMIT ? OFFSET ?`,
			"%"+filter+"%", filter, limit, offset,
		)
	} else {
		rows, err = s.db.Query(
			`SELECT id, execution_timestamp, script_id, executed_script, original_script, placeholder_values, working_directory, script_object_definition, executed_script_hash, original_script_hash
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

func (s *ExecutionHistoryService) GetScriptHistory(scriptID string, limit int) ([]ExecutionRecord, error) {
	rows, err := s.db.Query(
		`SELECT id, execution_timestamp, script_id, executed_script, original_script, placeholder_values, working_directory, script_object_definition, executed_script_hash, original_script_hash
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
		if err := rows.Scan(
			&r.ID, &r.ExecutionTimestamp, &r.ScriptID, &r.ExecutedScript,
			&r.OriginalScript, &pvJSON, &r.WorkingDirectory,
			&r.ScriptObjectDefinition, &r.ExecutedScriptHash, &r.OriginalScriptHash,
		); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(pvJSON), &r.PlaceholderValues); err != nil {
			r.PlaceholderValues = map[string]string{}
		}
		var scriptObj struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal([]byte(r.ScriptObjectDefinition), &scriptObj); err == nil {
			r.ScriptName = scriptObj.Name
		}
		records = append(records, r)
	}
	return records, nil
}

func BuildExecutionRecord(script *entities.Script, executedScript, originalScript string, placeholderValues map[string]string, workingDir string) ExecutionRecord {
	scriptJSON, _ := json.Marshal(script)
	return ExecutionRecord{
		ScriptID:              script.ID,
		ExecutedScript:        executedScript,
		OriginalScript:        originalScript,
		PlaceholderValues:     placeholderValues,
		WorkingDirectory:      workingDir,
		ScriptObjectDefinition: string(scriptJSON),
	}
}
