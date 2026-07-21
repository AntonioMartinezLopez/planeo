-- +goose Up
-- +goose StatementBegin

-- Just-in-time user provisioning (see
-- docs/superpowers/specs/2026-07-20-jit-user-provisioning-design.md) upserts
-- a users row keyed on uuid via ON CONFLICT (uuid) DO NOTHING, which requires
-- a unique constraint on that column - it previously had none.
CREATE UNIQUE INDEX users_uuid_idx ON users (uuid);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS users_uuid_idx;
-- +goose StatementEnd
