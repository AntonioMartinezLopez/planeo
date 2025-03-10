-- +goose Up
-- +goose StatementBegin

-- define tables
CREATE TABLE settings (
    id INTEGER PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
    host TEXT NOT NULL,
    port INTEGER NOT NULL,
    username TEXT NOT NULL,
    password TEXT NOT NULL,
    organization_id INTEGER NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Define triggers
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = current_timestamp;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER update_settings_updated_at
BEFORE UPDATE ON settings
FOR EACH ROW
EXECUTE FUNCTION update_updated_at_column();

-- add default data
INSERT INTO settings (host, port, username, password, organization_id)
VALUES ('localhost', 3143, 'test@test.de', 'test', 1);

-- +goose StatementEnd

-- +goose down
-- +goose StatementBegin
DROP TABLE IF EXISTS settings;
DROP TRIGGER IF EXISTS update_settings_updated_at ON settings;
-- +goose StatementEnd