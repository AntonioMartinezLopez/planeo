-- +goose Up
-- +goose StatementBegin

-- define tables
CREATE TABLE organizations (
    id INTEGER PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
    name TEXT NOT NULL UNIQUE,
    address TEXT NOT NULL,
    email TEXT NOT NULL,
    iam_organization_id TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TYPE role AS ENUM ('Admin', 'Planner', 'User');
CREATE TABLE users (
    id INTEGER PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
    username text,
    first_name text NOT NULL,
    last_name text NOT NULL,
    email text NOT NULL,
    iam_user_id text NOT NULL,
    organization_id INTEGER REFERENCES organizations(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE categories (
    id INTEGER PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
    label TEXT NOT NULL,
    color TEXT NOT NULL,
    label_description TEXT NOT NULL,
    organization_id INTEGER REFERENCES organizations(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE requests (
    id INTEGER PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
    text TEXT,
    subject TEXT,
    name TEXT,
    email TEXT,
    address TEXT,
    telephone TEXT,
    raw TEXT,
    closed BOOLEAN DEFAULT FALSE,
    reference_id TEXT,
    category_id INTEGER REFERENCES categories(id) ON DELETE SET NULL DEFAULT NULL,
    organization_id INTEGER REFERENCES organizations(id) ON DELETE CASCADE,
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

CREATE TRIGGER update_requests_updated_at
BEFORE UPDATE ON requests
FOR EACH ROW
EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_categories_updated_at
BEFORE UPDATE ON categories
FOR EACH ROW
EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_organizations_updated_at
BEFORE UPDATE ON organizations
FOR EACH ROW
EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_users_updated_at
BEFORE UPDATE ON users
FOR EACH ROW
EXECUTE FUNCTION update_updated_at_column();

-- add default data
INSERT INTO organizations (name, address, email, iam_organization_id) VALUES
('local', '456 Local St, Hometown', 'contact@local.org', 'local');

INSERT INTO categories (label, color, label_description, organization_id) VALUES
('Installation', '#FF5733', 'Request for installing new equipment', 1),
('Maintenance', '#FFC733', 'Request for maintenance', 1),
('Repair', '#33FF57', 'Request for repair', 1),
('Order', '#33FFC7', 'Request for ordering new equipment', 1),
('Support', '#3357FF', 'Customer support inquiries', 1),
('Other', '#FF33C7', 'Other requests', 1);

INSERT INTO requests (text, subject, name, email, address, telephone, category_id, organization_id, reference_id, raw) VALUES
('Install new electrical outlets in the conference room', 'Installation electrics in conference room', 'Emily Clark', 'emily.clark@example.com', '123 Main St, Springfield', '555-1234', 1, 1, '1234', ''),
('Routine maintenance of the electrical wiring in the main office', 'Request: Maintenance electrical wiring' ,'Michael Scott', 'michael.scott@example.com', '456 Elm St, Scranton', '555-5678', 2, 1, '1234', ''),
('Repair the broken light fixtures in the hallway', 'Request for fixing broken light fixtures in hallway' ,'Sarah Lee', 'sarah.lee@example.com', '789 Oak St, Metropolis', '555-8765', 3, 1, '1234', ''),
('Order new circuit breakers for the electrical panel', 'Order: Circuit breakers No.PW-44021' ,'David Wilson', 'david.wilson@example.com', '101 Pine St, Gotham', '555-4321', 4, 1, '1234', ''),
('Customer support for troubleshooting a power outage issue', 'Customer support needed for outage problem' ,'Laura Martinez', 'laura.martinez@example.com', '202 Maple St, Star City', '555-6789', 5, 1, '1234', '');

INSERT INTO "users" ("username", "first_name", "last_name", "email", "iam_user_id", "organization_id") VALUES 
('admin', 'admin', 'admin', 'admin@local.de', '7c806e52-e7cc-484b-843b-1242046590dc', 1),
('planner', 'planner', 'planner', 'planner@local.de', '146b3857-090e-453d-b1e6-8cdfbb1a6dcb', 1),
('user', 'User', 'User', 'user@local.de', 'd7eddb93-254e-4482-9a53-f31a5975dd1d', 1);
-- +goose StatementEnd

-- +goose down
-- +goose StatementBegin
DROP TABLE IF EXISTS requests;
DROP TABLE IF EXISTS categories;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS organizations;
DROP TYPE role;
DROP TRIGGER IF EXISTS update_requests_updated_at ON requests;
DROP TRIGGER IF EXISTS update_categories_updated_at ON categories;
DROP TRIGGER IF EXISTS update_organizations_updated_at ON organizations;
DROP FUNCTION IF EXISTS update_updated_at_column;
-- +goose StatementEnd