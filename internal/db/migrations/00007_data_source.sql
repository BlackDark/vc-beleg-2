-- +goose NO TRANSACTION
-- +goose Up
-- Receipts can be created from uploaded JSON data (source 'data'). SQLite
-- cannot change a CHECK constraint, so the table is rebuilt. selected_receipts
-- references receipts with ON DELETE CASCADE: dropping the old table with
-- foreign keys on would delete the selections, so they are switched off for
-- the rebuild (SQLite's documented procedure; the pool has one connection).
PRAGMA foreign_keys = OFF;
BEGIN;
-- Their conditions read receipts, which is dropped below; recreated afterwards.
DROP TRIGGER selected_receipt_eligible_insert;
DROP TRIGGER selected_receipt_eligible_update;
CREATE TABLE receipts_new (
    id                 INTEGER PRIMARY KEY,
    employee_id        INTEGER NOT NULL REFERENCES employees(id),
    status             TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'checked')),
    source             TEXT NOT NULL CHECK (source IN ('upload', 'mail', 'folder', 'data')),
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
    extracting         INTEGER NOT NULL DEFAULT 0 CHECK (extracting IN (0, 1)),
    UNIQUE (employee_id, number_year, number_seq),
    CHECK (status = 'pending' OR (date IS NOT NULL AND total IS NOT NULL AND number IS NOT NULL))
);
INSERT INTO receipts_new(id, employee_id, status, source, number, number_year, number_seq, sha256, filename, mime, size, pages, storage_key, preview_key, date, total, merchant, note, extraction, extraction_json, mail_message_id, mail_from, created_at, checked_at, extracting)
    SELECT id, employee_id, status, source, number, number_year, number_seq, sha256, filename, mime, size, pages, storage_key, preview_key, date, total, merchant, note, extraction, extraction_json, mail_message_id, mail_from, created_at, checked_at, extracting FROM receipts;
DROP TABLE receipts;
ALTER TABLE receipts_new RENAME TO receipts;
CREATE INDEX receipts_employee_date ON receipts(employee_id, date);
CREATE INDEX receipts_sha ON receipts(employee_id, sha256);
CREATE INDEX receipts_extraction_queue ON receipts(extraction, extracting, id);
-- +goose StatementBegin
CREATE TRIGGER clear_ineligible_receipt_selection AFTER UPDATE OF status, date ON receipts
WHEN NEW.status <> 'checked' OR NEW.date IS NOT OLD.date
BEGIN
    DELETE FROM selected_receipts WHERE receipt_id = OLD.id;
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER selected_receipt_eligible_insert BEFORE INSERT ON selected_receipts
WHEN NOT EXISTS (SELECT 1 FROM receipts WHERE id = NEW.receipt_id
    AND employee_id = NEW.employee_id AND date = NEW.date AND status = 'checked')
BEGIN
    SELECT RAISE(ABORT, 'selected receipt must be checked and belong to this employee and date');
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER selected_receipt_eligible_update BEFORE UPDATE ON selected_receipts
WHEN NOT EXISTS (SELECT 1 FROM receipts WHERE id = NEW.receipt_id
    AND employee_id = NEW.employee_id AND date = NEW.date AND status = 'checked')
BEGIN
    SELECT RAISE(ABORT, 'selected receipt must be checked and belong to this employee and date');
END;
-- +goose StatementEnd
COMMIT;
PRAGMA foreign_keys = ON;

-- +goose Down
SELECT 1;
