# planeo — Codebase Guide

AI-driven process management platform for service providers. Incoming requests (e.g., emails) are classified and enriched by an LLM, then routed to operators via a web UI.

## Monorepo layout

```
services/core/     ← main Go service (hexagonal architecture — canonical reference)
services/email/    ← email ingestion service (pre-hexagonal structure)
web/               ← Nuxt 3 frontend
libs/              ← shared Go libraries (logger, middlewares, events, db, …)
dev/               ← docker-compose + dev scripts
auth/              ← Keycloak realm configs
```

Single `go.mod` at the root (`module planeo`). Go 1.24.5.

## Build tool

[Task](https://taskfile.dev) — run `task --list` for all targets. Key tasks:

| Task | What it does |
|------|-------------|
| `task setup` | Install deps, copy `.env` templates |
| `task up` | Start Docker services + run DB migrations |
| `task down` | Stop all Docker services and wipe volumes |
| `task run:core` | Start core with Air hot-reload |
| `task run:email` | Start email service with Air hot-reload |
| `task login` | Obtain dev tokens (admin/planner/user) |
| `task test:core:unit` | Unit tests only (`-short`) |
| `task test:core:integration` | Integration tests (spins up testcontainers) |
| `task migrate:core` / `task migrate:email` | Run goose migrations |
| `task web:dev` | Start Nuxt dev server |
| `task web:generate-client` | Regenerate OpenAPI client for frontend |

## Core service — hexagonal architecture

`services/core` is the architectural reference. Every new service must follow this structure.

### Directory structure

```
services/core/
├── cmd/main.go                        ← entry point; wires all dependencies
├── internal/
│   ├── config/config.go               ← env-var config (godotenv)
│   ├── domain/<domain>/               ← pure domain layer (NO external imports)
│   │   ├── model.go                   ← domain models (plain Go structs)
│   │   ├── ports.go                   ← Service + Repository interfaces
│   │   ├── service.go                 ← domain logic (implements Service interface)
│   │   ├── errors.go                  ← domain-specific sentinel errors
│   │   ├── service_test.go            ← unit tests (use mocks)
│   │   └── mocks/                     ← mockery-generated mocks
│   ├── infra/
│   │   ├── postgres/                  ← driven adapter: implements Repository ports
│   │   │   ├── client.go
│   │   │   ├── <domain>_repository.go
│   │   │   └── migrations/            ← goose SQL migrations
│   │   ├── keycloak/                  ← driven adapter: implements IAM port
│   │   ├── llm/                       ← driven adapter: Mistral via langchaingo
│   │   ├── events/                    ← driving adapter: NATS subscriber
│   │   └── rest/                      ← driving adapter: HTTP server
│   │       ├── server.go              ← router setup, middleware wiring
│   │       └── api/v1/<domain>/
│   │           ├── handler.go         ← HTTP handler (maps DTOs ↔ domain types)
│   │           └── dto_*.go           ← request/response structs (Huma v2)
│   └── test/
│       ├── <domain>/<domain>_test.go  ← integration tests per domain
│       └── utils/                     ← testcontainer setup (Postgres + Keycloak)
└── pkg/
    └── keycloak/                      ← low-level Keycloak admin HTTP client
```

### Hexagonal architecture rules

**Domain layer** (`internal/domain/`) is the core; it must have zero infrastructure imports.

Each domain package contains exactly these files:
- `ports.go` — defines the `Service` interface (exposed to driving adapters) and `Repository` / `IAM` interfaces (required from driven adapters)
- `model.go` — plain Go structs; use `db:` struct tags for pgx scanning
- `service.go` — unexported `service` struct implementing `Service`; receives its ports via constructor
- `errors.go` — sentinel errors (`var ErrFoo = errors.New("…")`)

**Wiring happens exclusively in `cmd/main.go`**. No package should import another layer's concrete type.

**Data flow:**
```
HTTP Request
  → REST Handler (infra/rest) — maps DTO → domain type
    → Domain Service (domain/) — pure business logic
      → Repository/IAM (infra/postgres | infra/keycloak) — talks to external systems
```

**DTOs exist only at the HTTP boundary** (`api/v1/<domain>/dto_*.go`). Domain models flow between the service and repository layers directly.

### Adding a new domain (checklist)

1. Create `internal/domain/<name>/` with `model.go`, `ports.go`, `service.go`, `errors.go`
2. Implement the repository in `internal/infra/postgres/<name>_repository.go` (method on `*Client`)
3. Add handler + DTOs in `internal/infra/rest/api/v1/<name>/handler.go`
4. Register handler in `internal/infra/rest/server.go` (`InitRoutes`)
5. Wire service in `cmd/main.go`
6. Generate mocks: `cd services/core && mockery`
7. Write unit tests in `internal/domain/<name>/service_test.go` using mocks
8. Write integration tests in `internal/test/<name>/<name>_test.go` using `NewIntegrationTestEnvironment`

### REST layer conventions

- Framework: [Huma v2](https://huma.rocks) on top of chi
- All routes are under `/api/v1/organizations/{organizationId}/…`
- Each handler file has a `RegisterRoutes(api huma.API, permissions middlewares.PermissionMiddlewareConfig)` method
- Permission checks use `permissions.Apply("<resource>", "<action>")` as Huma middleware (e.g., `"request", "read"`)
- Error conversion: `NewHTTPError(err)` in `internal/infra/rest/api/errors.go`
- OpenAPI spec is auto-generated at startup to `docs/open-api-specs.yaml`

### Authentication & authorisation

- **IAM**: Keycloak (JWT bearer, OIDC)
- **Middleware stack**: JWT validation → organization ownership check → RBAC permission check
- **Roles**: `admin`, `planner`, `user` (Keycloak client roles)
- **Multi-tenant**: all data is scoped to `organization_id`; `OrganizationCheckMiddleware` validates the JWT's organization against the URL parameter

### Event-driven flow (NATS)

The email service publishes `email.received` events to NATS. The core service subscribes in `internal/infra/events/events.go`:

1. `EmailCreatedPayload` received
2. `CreateRequest` called on the domain service
3. LLM extracts structured fields (`infra/llm/request_field_extractor.go`)
4. LLM classifies into a category (`infra/llm/request_classifier.go`) using Mistral
5. Request updated with enriched data

### Database

- PostgreSQL via pgx/v5 (no ORM)
- Migrations with [goose](https://github.com/pressly/goose); files in `internal/infra/postgres/migrations/`
- Named args (`pgx.NamedArgs`) in all queries
- `pgx.RowToStructByName` for scanning — requires matching `db:` tags on domain models
- Migrations directory for the email service is at `services/email/db/migrations/`

### Testing

**Unit tests** (`-short` flag):
- Live in `internal/domain/<name>/service_test.go`
- Use mockery-generated mocks from `internal/domain/<name>/mocks/`
- Run: `task test:core:unit`

**Integration tests**:
- Live in `internal/test/<name>/<name>_test.go`
- Use `NewIntegrationTestEnvironment(t)` which spins up real Postgres and Keycloak testcontainers
- Run: `task test:core:integration` (sequential: `-p 1`)
- Testcontainers are cleaned up automatically via `t.Cleanup`

**Mock generation**:
```bash
cd services/core && mockery
```
Config: `services/core/.mockery.yml`

## Shared libraries (`libs/`)

| Package | Purpose |
|---------|---------|
| `libs/logger` | zerolog-based structured logging with context propagation |
| `libs/middlewares` | Auth (JWT/JWKS), CORS, logging, org validation, permission middleware |
| `libs/events` | NATS event service + `EmailCreatedPayload` type |
| `libs/huma_utils` | `WithAuth` helper to attach auth requirements to Huma operations |
| `libs/db` | Shared DB utilities |
| `libs/errors` | Shared error types |
| `libs/jwks` | JWKS key fetching/caching |

## Frontend (`web/`)

- Nuxt 3, TypeScript
- API client auto-generated from core's OpenAPI spec: `task web:generate-client`
- Dev server: `task web:dev`

## Environment variables

Copy from templates before first run:
```
dev/.env.template           → dev/.env
services/core/.env.template → services/core/.env
services/email/.env.template → services/email/.env
services/core/internal/infra/postgres/.envrc.template → …/.envrc  (for goose)
```

Key env vars for core: `HOST`, `PORT`, `NATS_URL`, `DB_*`, `KC_*` (see `internal/config/config.go`).

## Dev credentials

| Username | Role | Password |
|----------|------|----------|
| `admin@local.de` | Admin | `admin` |
| `planner@local.de` | Planner | `planner` |
| `user@local.de` | User | `user` |

Swagger UI: `http://localhost:8000/api/docs` (while dev environment is running).
