-- +goose Up
CREATE TABLE users (
    id             INTEGER PRIMARY KEY,
    username       TEXT UNIQUE,
    name           TEXT NOT NULL,
    email          TEXT NOT NULL DEFAULT '',
    password_hash  TEXT,
    oidc_issuer    TEXT,
    oidc_subject   TEXT,
    theme          TEXT NOT NULL DEFAULT 'dark',
    created_at     TEXT NOT NULL,
    UNIQUE (oidc_issuer, oidc_subject)
);

CREATE TABLE sessions (
    token_hash  TEXT PRIMARY KEY,
    user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    method      TEXT NOT NULL,
    created_at  TEXT NOT NULL,
    expires_at  TEXT NOT NULL
);
CREATE INDEX sessions_user ON sessions(user_id);

CREATE TABLE login_failures (
    key TEXT NOT NULL,
    at  TEXT NOT NULL
);
CREATE INDEX login_failures_key ON login_failures(key, at);

CREATE TABLE companies (
    id             INTEGER PRIMARY KEY,
    name           TEXT NOT NULL DEFAULT '',
    address        TEXT NOT NULL DEFAULT '',
    state          TEXT NOT NULL DEFAULT 'BW',
    church_tax_bp  INTEGER NOT NULL DEFAULT 0 CHECK (church_tax_bp BETWEEN 0 AND 1000),
    legal_basis    TEXT NOT NULL DEFAULT '',
    created_at     TEXT NOT NULL
);

CREATE TABLE employees (
    id          INTEGER PRIMARY KEY,
    company_id  INTEGER NOT NULL REFERENCES companies(id),
    user_id     INTEGER NOT NULL UNIQUE REFERENCES users(id),
    name        TEXT NOT NULL,
    tax_model   TEXT NOT NULL DEFAULT 'A' CHECK (tax_model IN ('A', 'B', 'C')),
    created_at  TEXT NOT NULL
);

CREATE TABLE receipts (
    id                 INTEGER PRIMARY KEY,
    employee_id        INTEGER NOT NULL REFERENCES employees(id),
    status             TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'checked')),
    source             TEXT NOT NULL CHECK (source IN ('upload', 'mail')),
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
CREATE INDEX receipts_employee_date ON receipts(employee_id, date);
CREATE INDEX receipts_sha ON receipts(employee_id, sha256);

-- Dates whose receipts the employee combined into one meal.
CREATE TABLE combined_days (
    employee_id INTEGER NOT NULL REFERENCES employees(id),
    date        TEXT NOT NULL,
    PRIMARY KEY (employee_id, date)
);

CREATE TABLE months (
    id             INTEGER PRIMARY KEY,
    employee_id    INTEGER NOT NULL REFERENCES employees(id),
    month          TEXT NOT NULL,
    status         TEXT NOT NULL CHECK (status IN ('open', 'closed', 'paid')),
    paid_on        TEXT,
    reopen_reason  TEXT NOT NULL DEFAULT '',
    UNIQUE (employee_id, month)
);

CREATE TABLE reports (
    id             INTEGER PRIMARY KEY,
    month_id       INTEGER NOT NULL REFERENCES months(id),
    version        INTEGER NOT NULL,
    snapshot_json  TEXT NOT NULL,
    payout         INTEGER NOT NULL,
    pdf_key        TEXT NOT NULL,
    pdf_sha256     TEXT NOT NULL,
    reopen_reason  TEXT NOT NULL DEFAULT '',
    created_at     TEXT NOT NULL,
    UNIQUE (month_id, version)
);

CREATE TABLE audit_log (
    id         INTEGER PRIMARY KEY,
    user_id    INTEGER,
    entity     TEXT NOT NULL,
    entity_id  INTEGER,
    action     TEXT NOT NULL,
    reason     TEXT NOT NULL DEFAULT '',
    data_json  TEXT NOT NULL DEFAULT '{}',
    at         TEXT NOT NULL
);
CREATE INDEX audit_user ON audit_log(user_id, at);

-- +goose Down
SELECT 1;
