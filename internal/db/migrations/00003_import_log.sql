-- +goose Up
-- What happened to each incoming file and receipt reading, shown in the
-- settings; entries older than 90 days are dropped.
CREATE TABLE import_log (
    id          INTEGER PRIMARY KEY,
    employee_id INTEGER NOT NULL REFERENCES employees(id),
    at          TEXT NOT NULL,
    source      TEXT NOT NULL CHECK (source IN ('upload', 'mail', 'folder', 'reader')),
    filename    TEXT NOT NULL DEFAULT '',
    outcome     TEXT NOT NULL CHECK (outcome IN ('imported', 'duplicate', 'rejected', 'read', 'read_failed')),
    message     TEXT NOT NULL DEFAULT '',
    receipt_id  INTEGER
);
CREATE INDEX import_log_employee_at ON import_log(employee_id, at);

-- +goose Down
DROP TABLE import_log;
