-- +goose Up
-- Keep existing receipt rows, selection references and extraction values intact.
ALTER TABLE receipts ADD COLUMN extracting INTEGER NOT NULL DEFAULT 0 CHECK (extracting IN (0, 1));
CREATE INDEX receipts_extraction_queue ON receipts(extraction, extracting, id);

-- +goose Down
SELECT 1;
