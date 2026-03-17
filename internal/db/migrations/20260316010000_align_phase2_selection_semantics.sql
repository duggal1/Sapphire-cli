-- +goose Up
-- +goose StatementBegin
ALTER TABLE memory_stage1_outputs ADD COLUMN source_updated_at INTEGER NOT NULL DEFAULT 0;
ALTER TABLE memory_stage1_outputs ADD COLUMN generated_at INTEGER NOT NULL DEFAULT 0;
ALTER TABLE memory_stage1_outputs ADD COLUMN usage_count INTEGER NOT NULL DEFAULT 0;
ALTER TABLE memory_stage1_outputs ADD COLUMN last_usage INTEGER;
ALTER TABLE memory_stage1_outputs ADD COLUMN selected_for_phase2 INTEGER NOT NULL DEFAULT 0;
ALTER TABLE memory_stage1_outputs ADD COLUMN selected_for_phase2_source_updated_at INTEGER;

UPDATE memory_stage1_outputs
SET source_updated_at = CASE
        WHEN source_updated_at = 0 THEN updated_at
        ELSE source_updated_at
    END,
    generated_at = CASE
        WHEN generated_at = 0 THEN updated_at
        ELSE generated_at
    END,
    last_usage = COALESCE(last_usage, used_at);

CREATE INDEX IF NOT EXISTS idx_memory_stage1_outputs_phase2_selection
    ON memory_stage1_outputs (selected_for_phase2, source_updated_at DESC, session_id DESC);
CREATE INDEX IF NOT EXISTS idx_memory_stage1_outputs_usage
    ON memory_stage1_outputs (usage_count DESC, last_usage DESC, source_updated_at DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_memory_stage1_outputs_usage;
DROP INDEX IF EXISTS idx_memory_stage1_outputs_phase2_selection;
-- +goose StatementEnd
