-- +goose Up
-- Deleting an unarchived receipt must never make its number available again.
CREATE TABLE receipt_sequences (
    employee_id INTEGER NOT NULL REFERENCES employees(id),
    year        INTEGER NOT NULL,
    last_seq    INTEGER NOT NULL,
    PRIMARY KEY (employee_id, year)
);
INSERT INTO receipt_sequences(employee_id, year, last_seq)
SELECT employee_id, number_year, max(number_seq)
FROM receipts WHERE number_year IS NOT NULL GROUP BY employee_id, number_year;

-- +goose Down
SELECT 1;
