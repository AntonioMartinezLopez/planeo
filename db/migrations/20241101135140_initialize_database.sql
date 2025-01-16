-- +goose Up
-- +goose StatementBegin
CREATE TYPE role AS ENUM ('Admin', 'Planner', 'User');
CREATE TABLE users (
    id INTEGER PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
    username text,
    first_name text NOT NULL,
    last_name text NOT NULL,
    email text NOT NULL,
    iam_user_id text NOT NULL,
    organization text NOT NULL
);

INSERT INTO "users" ("username", "first_name", "last_name", "email", "iam_user_id", "organization") VALUES 
('admin', 'admin', 'admin', 'admin@local.de', '7c806e52-e7cc-484b-843b-1242046590dc', 'local'),
('planner', 'planner', 'planner', 'planner@local.de', '146b3857-090e-453d-b1e6-8cdfbb1a6dcb', 'local'),
('user', 'User', 'User', 'user@local.de', 'd7eddb93-254e-4482-9a53-f31a5975dd1d', 'local');
-- +goose StatementEnd

-- +goose down
-- +goose StatementBegin
DROP TABLE users;
DROP TYPE role;
-- +goose StatementEnd