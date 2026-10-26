-- +goose Up
CREATE TABLE selected_receipts (
    employee_id INTEGER NOT NULL REFERENCES employees(id),
    date        TEXT NOT NULL,
    receipt_id  INTEGER NOT NULL UNIQUE REFERENCES receipts(id) ON DELETE CASCADE,
    PRIMARY KEY (employee_id, date)
);

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

-- +goose StatementBegin
CREATE TRIGGER clear_ineligible_receipt_selection AFTER UPDATE OF status, date ON receipts
WHEN NEW.status <> 'checked' OR NEW.date IS NOT OLD.date
BEGIN
    DELETE FROM selected_receipts WHERE receipt_id = OLD.id;
END;
-- +goose StatementEnd

-- +goose Down
SELECT 1;
