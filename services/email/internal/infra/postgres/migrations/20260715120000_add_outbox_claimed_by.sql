-- +goose Up
-- +goose StatementBegin
ALTER TABLE outbox ADD COLUMN claimed_by TEXT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE outbox DROP COLUMN claimed_by;
-- +goose StatementEnd
