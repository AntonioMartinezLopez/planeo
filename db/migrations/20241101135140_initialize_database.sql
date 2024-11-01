-- +goose Up
-- +goose StatementBegin
CREATE TABLE users (
    id INTEGER PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
    username text,
    first_name text NOT NULL,
    last_name text NOT NULL,
    email text NOT NULL,
    keycloak_id text NOT NULL
);

INSERT INTO "users" ("username", "first_name", "last_name", "email", "keycloak_id") VALUES 
('admin', 'admin', 'admin', 'admin@local.de', '7c806e52-e7cc-484b-843b-1242046590dc'),
('planner', 'planner', 'planner', 'planner@local.de', '146b3857-090e-453d-b1e6-8cdfbb1a6dcb'),
('user', 'User', 'User', 'user@local.de', 'd7eddb93-254e-4482-9a53-f31a5975dd1d');
-- +goose StatementEnd

-- +goose down
-- +goose StatementBegin
DROP TABLE users;
-- +goose StatementEnd