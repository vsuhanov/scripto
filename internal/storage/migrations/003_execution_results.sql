ALTER TABLE execution_history ADD COLUMN invoked_from TEXT NOT NULL DEFAULT '';
UPDATE execution_history SET invoked_from = working_directory;
ALTER TABLE execution_history ADD COLUMN exit_code INTEGER;
ALTER TABLE execution_history ADD COLUMN finished_at INTEGER NOT NULL DEFAULT 0;
CREATE INDEX IF NOT EXISTS idx_execution_invoked_from ON execution_history(invoked_from);
