-- +goose Up
-- With proof, every meal day on a working day counts (no 15-day limit).
ALTER TABLE months ADD COLUMN prove_working_days INTEGER NOT NULL DEFAULT 0;

-- Working-day calendar entries; dates without one follow the weekday rule.
CREATE TABLE day_marks (
    employee_id INTEGER NOT NULL REFERENCES employees(id),
    date        TEXT NOT NULL,
    mark        TEXT NOT NULL CHECK (mark IN ('vacation', 'sick', 'off', 'worked')),
    PRIMARY KEY (employee_id, date)
);

-- +goose Down
DROP TABLE day_marks;
ALTER TABLE months DROP COLUMN prove_working_days;
