-- +goose Up
-- Receipts can come from a watched folder (WebDAV). SQLite cannot change a
-- CHECK constraint, so the table is rebuilt; no other table references it.
CREATE TABLE receipts_new (
    id                 INTEGER PRIMARY KEY,
    employee_id        INTEGER NOT NULL REFERENCES employees(id),
    status             TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'checked')),
    source             TEXT NOT NULL CHECK (source IN ('upload', 'mail', 'folder')),
    number             TEXT,
    number_year        INTEGER,
    number_seq         INTEGER,
    sha256             TEXT NOT NULL,
    filename           TEXT NOT NULL,
    mime               TEXT NOT NULL,
    size               INTEGER NOT NULL,
    pages              INTEGER NOT NULL DEFAULT 1,
    storage_key        TEXT NOT NULL,
    preview_key        TEXT NOT NULL,
    date               TEXT,
    total              INTEGER CHECK (total IS NULL OR total > 0),
    merchant           TEXT NOT NULL DEFAULT '',
    note               TEXT NOT NULL DEFAULT '',
    extraction         TEXT NOT NULL DEFAULT 'off' CHECK (extraction IN ('off', 'pending', 'done', 'failed')),
    extraction_json    TEXT NOT NULL DEFAULT '{}',
    mail_message_id    TEXT NOT NULL DEFAULT '',
    mail_from          TEXT NOT NULL DEFAULT '',
    created_at         TEXT NOT NULL,
    checked_at         TEXT,
    UNIQUE (employee_id, number_year, number_seq),
    CHECK (status = 'pending' OR (date IS NOT NULL AND total IS NOT NULL AND number IS NOT NULL))
);
INSERT INTO receipts_new SELECT * FROM receipts;
DROP TABLE receipts;
ALTER TABLE receipts_new RENAME TO receipts;
CREATE INDEX receipts_employee_date ON receipts(employee_id, date);
CREATE INDEX receipts_sha ON receipts(employee_id, sha256);

-- +goose Down
SELECT 1;
