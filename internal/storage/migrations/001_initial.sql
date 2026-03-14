CREATE TABLE IF NOT EXISTS execution_history (
    id TEXT PRIMARY KEY,
    execution_timestamp INTEGER NOT NULL,
    script_id TEXT NOT NULL,
    executed_script TEXT NOT NULL,
    original_script TEXT NOT NULL,
    placeholder_values TEXT NOT NULL DEFAULT '{}',
    working_directory TEXT NOT NULL DEFAULT '',
    script_object_definition TEXT NOT NULL DEFAULT '{}',
    executed_script_hash TEXT NOT NULL DEFAULT '',
    original_script_hash TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_script_id ON execution_history(script_id);
CREATE INDEX IF NOT EXISTS idx_execution_timestamp ON execution_history(execution_timestamp);
