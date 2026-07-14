-- +goose Up
-- +goose StatementBegin

CREATE TABLE mails (
    id              INTEGER PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
    message_id      TEXT NOT NULL,
    setting_id      INTEGER NOT NULL REFERENCES settings(id),
    organization_id INTEGER NOT NULL,
    subject         TEXT NOT NULL,
    sender          TEXT NOT NULL,
    body            TEXT NOT NULL,
    date            TIMESTAMPTZ NOT NULL,
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE (setting_id, message_id)
);

CREATE TABLE outbox (
    id           BIGINT PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
    mail_id      INTEGER NOT NULL REFERENCES mails(id),
    topic        TEXT NOT NULL,
    key          BYTEA,
    payload      BYTEA NOT NULL,
    status       TEXT NOT NULL DEFAULT 'pending',
    claimed_at   TIMESTAMPTZ,
    attempts     INTEGER NOT NULL DEFAULT 0,
    last_error   TEXT,
    processed_at TIMESTAMPTZ,
    failed_at    TIMESTAMPTZ,
    created_at   TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX outbox_pending_idx ON outbox (id) WHERE status IN ('pending', 'processing');

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS outbox_pending_idx;
DROP TABLE IF EXISTS outbox;
DROP TABLE IF EXISTS mails;
-- +goose StatementEnd
