-- +goose Up
-- Dates whose allowance is for breakfast or dinner instead of lunch.
CREATE TABLE day_meals (
    employee_id INTEGER NOT NULL REFERENCES employees(id),
    date        TEXT NOT NULL,
    meal        TEXT NOT NULL CHECK (meal IN ('breakfast', 'dinner')),
    PRIMARY KEY (employee_id, date)
);

-- +goose Down
DROP TABLE day_meals;
