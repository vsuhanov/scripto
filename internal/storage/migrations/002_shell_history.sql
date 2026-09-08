CREATE TABLE IF NOT EXISTS shell_history (
    id TEXT PRIMARY KEY,
    command TEXT NOT NULL DEFAULT '',
    working_directory TEXT NOT NULL DEFAULT '',
    started_at INTEGER NOT NULL DEFAULT 0,
    finished_at INTEGER NOT NULL DEFAULT 0,
    exit_code INTEGER,
    session_id TEXT NOT NULL DEFAULT '',
    source TEXT NOT NULL DEFAULT 'shell',
    saved_script_id TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_shell_history_started ON shell_history(started_at);
CREATE INDEX IF NOT EXISTS idx_shell_history_dir ON shell_history(working_directory);
