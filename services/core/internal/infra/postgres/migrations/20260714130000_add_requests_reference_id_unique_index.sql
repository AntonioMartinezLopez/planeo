-- +goose Up
-- +goose StatementBegin

-- Two requests in the same organization must not share the same
-- reference_id (the source email's Message-ID) — this is what makes
-- re-processing an inbox record after a failed MarkProcessed safe:
-- CreateRequest resolves to the existing row instead of creating a
-- duplicate. Requests without a reference_id (e.g. created manually via
-- the REST API, where it's never set and defaults to '') are excluded
-- by the WHERE clause and remain completely unconstrained.
CREATE UNIQUE INDEX requests_org_reference_id_idx ON requests (organization_id, reference_id) WHERE reference_id <> '';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS requests_org_reference_id_idx;
-- +goose StatementEnd
