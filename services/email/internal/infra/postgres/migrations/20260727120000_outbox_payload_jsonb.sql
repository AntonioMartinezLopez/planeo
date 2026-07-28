-- +goose Up
-- +goose StatementBegin
ALTER TABLE outbox ALTER COLUMN payload TYPE JSONB USING convert_from(payload, 'UTF8')::jsonb;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE outbox ALTER COLUMN payload TYPE BYTEA USING convert_to(payload::text, 'UTF8');
-- +goose StatementEnd
