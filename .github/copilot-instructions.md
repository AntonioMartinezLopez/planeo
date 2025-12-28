# Planeo - AI Coding Agent Instructions

## Architecture Overview

Planeo is an AI-driven process management platform with a **monorepo** structure:

- **Backend Services** (Go 1.24+): `services/core/` and `services/email/` - Each service has its own database migrations in `db/migrations/`
- **Shared Libraries** (Go): `libs/` - Reusable packages for API, auth, events, middlewares
- **Frontend** (Nuxt 4 + Vue 3): `web/` - SPA with shadcn-vue components
- **Auth**: Keycloak handles authentication; JWT validation via JWKS

**Key Data Flows:**
1. Frontend → Nuxt server proxy (`web/server/api/[...].ts`) → Go backend (injects Bearer token)
2. Email service → NATS JetStream events → Core service (async processing)
3. Backend uses **Huma v2** framework with Chi router for REST APIs

## Developer Commands

```bash
make setup          # Install deps, create .env files
make up             # Start Docker services + run DB migrations
make run core       # Start core service with Air hot-reload
make run email      # Start email service with Air hot-reload
make login          # Get test tokens (admin/planner/user)
make test core unit       # Unit tests only (-short flag)
make test core integration  # Integration tests with testcontainers
make down           # Stop all containers
```

Frontend: `cd web && npm run dev` | Generate API client: `npm run generate-client:core`

## Backend Patterns

### Resource Structure (`services/core/internal/resources/<resource>/`)
```
controller.go    # Route registration with Huma
service.go       # Business logic + interface definition
repository.go    # Database queries
dto/             # Input/output structs with Huma tags
models/          # Domain models
mocks/           # Auto-generated (mockery)
```

### Controller Pattern Example
```go
// Use humaUtils.WithAuth() to require bearer auth
huma.Register(r.api, humaUtils.WithAuth(huma.Operation{
    OperationID: "get-requests",
    Method:      http.MethodGet,
    Path:        "/organizations/{organizationId}/requests",
    Middlewares: huma.Middlewares{permissions.Apply("request", "read")},
}), func(ctx context.Context, input *dto.GetRequestsInput) (*dto.GetRequestsOutput, error) { ... })
```

### DTO Struct Tags (Huma)
```go
type Input struct {
    OrganizationId int  `path:"organizationId" doc:"ID of the organization"`
    PageSize       int  `query:"pageSize" required:"true" doc:"Number of items"`
    Body           struct {
        Name string `json:"name" example:"John"`
    }
}
```

### Testing
- **Unit tests**: Use `-short` flag, mock interfaces with mockery-generated mocks
- **Integration tests**: Use `utils.NewIntegrationTestEnvironment(t)` for testcontainers setup
- OpenAPI specs auto-generated to `services/core/docs/open-api-specs.yaml` on startup

## Frontend Patterns

### API Client Generation
Client is generated from OpenAPI spec using `@hey-api/openapi-ts`:
```bash
npm run generate-client:core  # Outputs to web/app/clients/core/
```

### Composables with TanStack Query
```typescript
// Example from usePaginatedRequests.ts
const { data } = useQuery({
    queryKey: ["get-requests", cursor.value, pageSize.value],
    queryFn: () => getRequests({
        composable: "$fetch",  // Use Nuxt's $fetch
        path: { organizationId },
        query: { pageSize: pageSize.value },
    }),
});
```

### UI Components
- Use shadcn-vue components from `web/app/components/ui/`
- Tailwind CSS 4 with `@tailwindcss/vite` plugin

## Event-Driven Communication

Services communicate via NATS JetStream:
```go
// Publishing (email service)
eventService.Publish(ctx, "events.email.received", payload)

// Subscribing (core service in internal/events/)
eventService.SubscribeEmailReceived(ctx, callback)
```

## Key Files to Reference
- [libs/api/api.go](libs/api/api.go) - API initialization pattern
- [services/core/internal/setup/app_factory.go](services/core/internal/setup/app_factory.go) - DI wiring
- [libs/middlewares/auth.go](libs/middlewares/auth.go) - JWT validation
- [web/app/clients/configuration/core.ts](web/app/clients/configuration/core.ts) - Frontend API config
