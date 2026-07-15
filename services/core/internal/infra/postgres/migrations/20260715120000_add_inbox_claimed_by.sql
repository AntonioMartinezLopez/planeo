-- +goose Up
-- +goose StatementBegin
ALTER TABLE inbox ADD COLUMN claimed_by TEXT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE inbox DROP COLUMN claimed_by;
-- +goose StatementEnd
