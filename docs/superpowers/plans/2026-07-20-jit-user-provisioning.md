# Just-In-Time User Provisioning Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the hardcoded-UUID coupling between Keycloak and Postgres's seed migration with just-in-time (JIT) provisioning of the Postgres `users` row on a caller's first authenticated request.

**Architecture:** A new middleware in `services/core/internal/infra/rest/`, registered right after `AuthMiddleware`, resolves the caller's organization from their JWT `groups` claim and calls a new `user.Service.EnsureProvisioned` method that idempotently upserts the Postgres row (`INSERT ... ON CONFLICT (uuid) DO NOTHING`). This removes the need to keep Keycloak's Terraform-assigned user IDs in sync with Postgres's seed data by hand.

**Tech Stack:** Go 1.24.5, pgx/v5, Huma v2, goose migrations, testify/mockery.

## Global Constraints

- Local-only scope: no `dev`/`stage`/`prod` changes (matches the parent IaC plan).
- Provisioning is best-effort and never blocks the request it runs alongside: any failure (empty `groups` claim, unresolvable organization, DB error) is logged and `next` is still called.
- Only single-organization-per-user is handled (`Groups[0]`); this is out of scope to generalize further per the design spec's non-goals.
- The new middleware lives in `services/core/internal/infra/rest/`, not the shared `libs/middlewares`, because it depends on the services/core-specific `user.Service`/`organization.Service` — the shared library must not import concrete service types from a specific service (hexagonal layering rule in this repo's CLAUDE.md).
- Reference design spec: `docs/superpowers/specs/2026-07-20-jit-user-provisioning-design.md`. If anything here conflicts with it, this plan is the more specific, later-verified source.

---

## File structure

```
services/core/internal/infra/postgres/migrations/
├── 20260720120000_add_users_uuid_unique_index.sql   # new
└── 20241101135140_initialize_database.sql            # modified (Task 6)

libs/middlewares/
├── claims.go          # modified
└── claims_test.go     # new

services/core/internal/domain/organization/
├── ports.go            # modified
├── service.go           # modified
└── service_test.go      # new

services/core/internal/domain/user/
├── ports.go            # modified
└── service_test.go      # modified

services/core/internal/infra/postgres/
├── organziation_repository.go   # modified (filename typo is pre-existing, not fixed here)
└── user_repository.go            # modified

services/core/internal/infra/rest/
├── user_provisioning.go   # new
└── server.go                # modified

services/core/internal/test/organization/
└── organization_test.go     # modified

dev/
└── sync_dev_user_ids.sh    # deleted (Task 7)

infra/environments/local/keycloak/
└── outputs.tf                # deleted (Task 7)

services/core/internal/test/testdata/
└── realm-users.json          # modified (Task 7)

Taskfile.yml                  # modified (Task 7)
```

Modified mocks (regenerated via `mockery`, not hand-written): `services/core/internal/domain/organization/mocks/*`, `services/core/internal/domain/user/mocks/*`.

---

### Task 1: Postgres schema — unique index on `users.uuid`

**Files:**
- Create: `services/core/internal/infra/postgres/migrations/20260720120000_add_users_uuid_unique_index.sql`

**Interfaces:**
- Produces: a unique index `users_uuid_idx` on `users(uuid)`, required by Task 4's `ON CONFLICT (uuid)` clause. `users.uuid` currently has no uniqueness constraint at all (`text NOT NULL` only, `20241101135140_initialize_database.sql:22`).

- [ ] **Step 1: Write the migration**

Create `services/core/internal/infra/postgres/migrations/20260720120000_add_users_uuid_unique_index.sql`:
```sql
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
```

- [ ] **Step 2: Apply and verify**

From the repo root:
```bash
task migrate:core
```
Expected: goose reports `OK   20260720120000_add_users_uuid_unique_index.sql` and migrates to that version.

Verify the index exists:
```bash
docker exec postgres psql -U planeo -d planeo -c "\d users" | grep users_uuid_idx
```
Expected: a line containing `users_uuid_idx` and `UNIQUE, btree (uuid)`.

- [ ] **Step 3: Verify the down migration works**

```bash
task migrate:core:down
docker exec postgres psql -U planeo -d planeo -c "\d users" | grep users_uuid_idx
```
Expected: no output (index dropped).

Re-apply so the environment is left migrated:
```bash
task migrate:core
```

- [ ] **Step 4: Commit**

```bash
git add services/core/internal/infra/postgres/migrations/20260720120000_add_users_uuid_unique_index.sql
git commit -m "feat(core): add unique index on users.uuid for JIT provisioning upsert"
```

---

### Task 2: JWT claims — expose `GivenName`/`FamilyName`/`PreferredUsername`

**Files:**
- Modify: `libs/middlewares/claims.go`
- Create: `libs/middlewares/claims_test.go`

**Interfaces:**
- Produces: `OauthAccessClaims.GivenName`, `.FamilyName`, `.PreferredUsername` (all `string`) — consumed by Task 5's middleware.

- [ ] **Step 1: Write the failing test**

Create `libs/middlewares/claims_test.go`:
```go
package middlewares_test

import (
	"encoding/json"
	"planeo/libs/middlewares"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOauthAccessClaimsUnmarshal(t *testing.T) {
	raw := []byte(`{
		"sub": "abc-123",
		"name": "admin admin",
		"given_name": "admin",
		"family_name": "Admin",
		"preferred_username": "admin",
		"email": "admin@local.de",
		"groups": ["/local"]
	}`)

	var claims middlewares.OauthAccessClaims
	err := json.Unmarshal(raw, &claims)

	assert.NoError(t, err)
	assert.Equal(t, "admin", claims.GivenName)
	assert.Equal(t, "Admin", claims.FamilyName)
	assert.Equal(t, "admin", claims.PreferredUsername)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./libs/middlewares/... -run TestOauthAccessClaimsUnmarshal -v`
Expected: compile error — `claims.GivenName undefined (type middlewares.OauthAccessClaims has no field or method GivenName)` (and similarly for the other two fields).

- [ ] **Step 3: Add the fields**

In `libs/middlewares/claims.go`, replace:
```go
type OauthAccessClaims struct {
	Sub            string   `json:"sub"`
	Name           string   `json:"name"`
	Email          string   `json:"email"`
	UserId         string   `json:"userid"`
	Issuer         string   `json:"iss"`
	Groups         []string `json:"groups"`
	ExpirationTime int64    `json:"exp"`
	Roles          []string `json:"roles"`
}
```
with:
```go
type OauthAccessClaims struct {
	Sub               string   `json:"sub"`
	Name              string   `json:"name"`
	GivenName         string   `json:"given_name"`
	FamilyName        string   `json:"family_name"`
	PreferredUsername string   `json:"preferred_username"`
	Email             string   `json:"email"`
	UserId            string   `json:"userid"`
	Issuer            string   `json:"iss"`
	Groups            []string `json:"groups"`
	ExpirationTime    int64    `json:"exp"`
	Roles             []string `json:"roles"`
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./libs/middlewares/... -run TestOauthAccessClaimsUnmarshal -v`
Expected: `PASS`.

- [ ] **Step 5: Commit**

```bash
git add libs/middlewares/claims.go libs/middlewares/claims_test.go
git commit -m "feat(middlewares): expose given_name/family_name/preferred_username on OauthAccessClaims"
```

---

### Task 3: `organization` domain — `GetOrganizationByIAMID`

**Files:**
- Modify: `services/core/internal/domain/organization/ports.go`
- Modify: `services/core/internal/domain/organization/service.go`
- Modify: `services/core/internal/infra/postgres/organziation_repository.go`
- Create: `services/core/internal/domain/organization/service_test.go`
- Regenerate: `services/core/internal/domain/organization/mocks/*`

**Interfaces:**
- Consumes: none beyond what already exists.
- Produces: `organization.Service.GetOrganizationByIAMID(ctx context.Context, iamOrganizationID string) (Organization, error)` — consumed by Task 5's middleware.

- [ ] **Step 1: Write the failing unit test**

Create `services/core/internal/domain/organization/service_test.go`:
```go
package organization_test

import (
	"context"
	. "planeo/services/core/internal/domain/organization"
	"planeo/services/core/internal/domain/organization/mocks"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOrganizationService(t *testing.T) {
	if !testing.Short() {
		t.Skip()
	}

	ctx := context.Background()

	t.Run("GetOrganizationByIAMID", func(t *testing.T) {
		t.Run("Should return error if repository fails", func(t *testing.T) {
			// Setup
			mockOrganizationRepository := mocks.NewMockOrganizationRepository(t)
			mockOrganizationRepository.EXPECT().GetOrganizationByIAMID(ctx, "local").Return(Organization{}, assert.AnError)
			organizationService := NewService(mockOrganizationRepository)

			// Act
			result, err := organizationService.GetOrganizationByIAMID(ctx, "local")

			// Assert
			assert.NotNil(t, err)
			assert.Equal(t, Organization{}, result)
		})

		t.Run("Should return organization if repository succeeds", func(t *testing.T) {
			// Setup
			expectedOrganization := Organization{Id: 1, Name: "local", IAMOrganizationID: "local"}
			mockOrganizationRepository := mocks.NewMockOrganizationRepository(t)
			mockOrganizationRepository.EXPECT().GetOrganizationByIAMID(ctx, "local").Return(expectedOrganization, nil)
			organizationService := NewService(mockOrganizationRepository)

			// Act
			result, err := organizationService.GetOrganizationByIAMID(ctx, "local")

			// Assert
			assert.Nil(t, err)
			assert.Equal(t, expectedOrganization, result)
		})
	})
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./services/core/internal/domain/organization/... -short -run TestOrganizationService -v`
Expected: compile error — `GetOrganizationByIAMID` is not a method on `Service`, and `mocks.NewMockOrganizationRepository(t).EXPECT()` has no `GetOrganizationByIAMID` (the mock doesn't have this method yet either — this is expected to fail at compile time until Steps 3-5 land).

- [ ] **Step 3: Add the method to `ports.go`**

In `services/core/internal/domain/organization/ports.go`, replace:
```go
type OrganizationRepository interface {
	GetOrganizationsByUserSub(ctx context.Context, userSub string) ([]Organization, error)
	GetOrganizationById(ctx context.Context, id int) (Organization, error)
}

type Service interface {
	GetOrganizationsByUserSub(ctx context.Context, userSub string) ([]Organization, error)
	GetOrganizationById(ctx context.Context, organizationId int) (Organization, error)
}
```
with:
```go
type OrganizationRepository interface {
	GetOrganizationsByUserSub(ctx context.Context, userSub string) ([]Organization, error)
	GetOrganizationById(ctx context.Context, id int) (Organization, error)
	GetOrganizationByIAMID(ctx context.Context, iamOrganizationID string) (Organization, error)
}

type Service interface {
	GetOrganizationsByUserSub(ctx context.Context, userSub string) ([]Organization, error)
	GetOrganizationById(ctx context.Context, organizationId int) (Organization, error)
	GetOrganizationByIAMID(ctx context.Context, iamOrganizationID string) (Organization, error)
}
```

- [ ] **Step 4: Implement the service method**

In `services/core/internal/domain/organization/service.go`, append after `GetOrganizationById`:
```go
func (s *service) GetOrganizationByIAMID(ctx context.Context, iamOrganizationID string) (Organization, error) {
	return s.organizationRepository.GetOrganizationByIAMID(ctx, iamOrganizationID)
}
```

- [ ] **Step 5: Implement the repository method**

In `services/core/internal/infra/postgres/organziation_repository.go`, append after `GetOrganizationById`:
```go
// GetOrganizationByIAMID returns the organization whose iam_organization_id
// matches the given Keycloak group/org name (e.g. "local").
func (c *Client) GetOrganizationByIAMID(ctx context.Context, iamOrganizationID string) (organization.Organization, error) {
	query := "SELECT * FROM organizations WHERE iam_organization_id = @iamOrganizationId"
	args := pgx.NamedArgs{"iamOrganizationId": iamOrganizationID}

	row, err := c.db.Query(ctx, query, args)
	if err != nil {
		return organization.Organization{}, NewDatabaseError("error querying database", err)
	}

	org, err := pgx.CollectOneRow(row, pgx.RowToStructByName[organization.Organization])
	if err != nil {
		return organization.Organization{}, NewDatabaseError("error collecting organization", err)
	}

	return org, nil
}
```

- [ ] **Step 6: Regenerate mocks**

```bash
cd services/core && mockery
```
Expected: `services/core/internal/domain/organization/mocks/organization_repository_mock.go` now has a `GetOrganizationByIAMID` method. Confirm:
```bash
grep -c "GetOrganizationByIAMID" internal/domain/organization/mocks/organization_repository_mock.go
```
Expected: non-zero.

- [ ] **Step 7: Run test to verify it passes**

Run: `go test ./services/core/internal/domain/organization/... -short -run TestOrganizationService -v`
Expected: `PASS`, both subtests green.

- [ ] **Step 8: Commit**

```bash
cd /Users/antoniomartinezlopez/dev/planeo
git add services/core/internal/domain/organization/ services/core/internal/infra/postgres/organziation_repository.go
git commit -m "feat(core): add organization.Service.GetOrganizationByIAMID"
```

---

### Task 4: `user` domain — `EnsureProvisioned`

**Files:**
- Modify: `services/core/internal/domain/user/ports.go`
- Modify: `services/core/internal/domain/user/service.go`
- Modify: `services/core/internal/infra/postgres/user_repository.go`
- Modify: `services/core/internal/domain/user/service_test.go`
- Regenerate: `services/core/internal/domain/user/mocks/*`

**Interfaces:**
- Consumes: Task 1's `users_uuid_idx` unique index (runtime dependency for `ON CONFLICT (uuid)` to be valid SQL).
- Produces: `user.Service.EnsureProvisioned(ctx context.Context, organizationId int, uuid, username, firstName, lastName, email string) error` — consumed by Task 5's middleware.

- [ ] **Step 1: Write the failing unit tests**

In `services/core/internal/domain/user/service_test.go`, insert this new `t.Run` block immediately before the final closing `}` of `TestUserService` (i.e. after the existing `t.Run("AssignRoles", ...)` block, still inside `func TestUserService(t *testing.T) { ... }`):
```go
	t.Run("EnsureProvisioned", func(t *testing.T) {
		t.Run("Should return error if EnsureUser fails", func(t *testing.T) {
			// Setup
			mockUserRepository := mocks.NewMockUserRepository(t)
			mockUserRepository.EXPECT().EnsureUser(ctx, testOrganizationId, "sub-123", "admin", "admin", "admin", "admin@local.de").Return(assert.AnError)
			userService := NewService(mockUserRepository, nil)

			// Act
			result := userService.EnsureProvisioned(ctx, testOrganizationId, "sub-123", "admin", "admin", "admin", "admin@local.de")

			// Assert
			assert.NotNil(t, result)
		})

		t.Run("Should return nil if EnsureUser succeeds", func(t *testing.T) {
			// Setup
			mockUserRepository := mocks.NewMockUserRepository(t)
			mockUserRepository.EXPECT().EnsureUser(ctx, testOrganizationId, "sub-123", "admin", "admin", "admin", "admin@local.de").Return(nil)
			userService := NewService(mockUserRepository, nil)

			// Act
			result := userService.EnsureProvisioned(ctx, testOrganizationId, "sub-123", "admin", "admin", "admin", "admin@local.de")

			// Assert
			assert.Nil(t, result)
		})
	})
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./services/core/internal/domain/user/... -short -run TestUserService/EnsureProvisioned -v`
Expected: compile error — `userService.EnsureProvisioned` and `mockUserRepository.EXPECT().EnsureUser` don't exist yet.

- [ ] **Step 3: Add the method to `ports.go`**

In `services/core/internal/domain/user/ports.go`, replace:
```go
type UserRepository interface {
	GetIamOrganizationIdentifier(ctx context.Context, organizationId int) (string, error)
	GetUsers(ctx context.Context, organizationId int) ([]User, error)
	SyncUsers(ctx context.Context, organizationId int, users []IAMUser) error
	CreateUser(ctx context.Context, organizationId int, uuid string, user NewUser) error
	DeleteUser(ctx context.Context, organizationId int, uuid string) error
	UpdateUser(ctx context.Context, organizationId int, uuid string, user UpdateUser) error
}

type Service interface {
	GetIAMUsers(ctx context.Context, organizationId int, sync bool) ([]IAMUser, error)
	CreateUser(ctx context.Context, organizationId int, newUser NewUser) error
	DeleteUser(ctx context.Context, organizationId int, uuid string) error
	UpdateUser(ctx context.Context, organizationId int, uuid string, user UpdateUser) error
	GetAvailableRoles(ctx context.Context) ([]Role, error)
	AssignRoles(ctx context.Context, organizationId int, uuid string, roles []Role) error
	GetUsers(ctx context.Context, organizationId int) ([]User, error)
	GetIAMUserByUuid(ctx context.Context, organizationId int, uuid string) (*IAMUser, error)
}
```
with:
```go
type UserRepository interface {
	GetIamOrganizationIdentifier(ctx context.Context, organizationId int) (string, error)
	GetUsers(ctx context.Context, organizationId int) ([]User, error)
	SyncUsers(ctx context.Context, organizationId int, users []IAMUser) error
	CreateUser(ctx context.Context, organizationId int, uuid string, user NewUser) error
	DeleteUser(ctx context.Context, organizationId int, uuid string) error
	UpdateUser(ctx context.Context, organizationId int, uuid string, user UpdateUser) error
	EnsureUser(ctx context.Context, organizationId int, uuid, username, firstName, lastName, email string) error
}

type Service interface {
	GetIAMUsers(ctx context.Context, organizationId int, sync bool) ([]IAMUser, error)
	CreateUser(ctx context.Context, organizationId int, newUser NewUser) error
	DeleteUser(ctx context.Context, organizationId int, uuid string) error
	UpdateUser(ctx context.Context, organizationId int, uuid string, user UpdateUser) error
	GetAvailableRoles(ctx context.Context) ([]Role, error)
	AssignRoles(ctx context.Context, organizationId int, uuid string, roles []Role) error
	GetUsers(ctx context.Context, organizationId int) ([]User, error)
	GetIAMUserByUuid(ctx context.Context, organizationId int, uuid string) (*IAMUser, error)
	EnsureProvisioned(ctx context.Context, organizationId int, uuid, username, firstName, lastName, email string) error
}
```

- [ ] **Step 4: Implement the service method**

In `services/core/internal/domain/user/service.go`, append after `GetUsers`:
```go
func (s *service) EnsureProvisioned(ctx context.Context, organizationId int, uuid, username, firstName, lastName, email string) error {
	return s.userRepository.EnsureUser(ctx, organizationId, uuid, username, firstName, lastName, email)
}
```

- [ ] **Step 5: Implement the repository method**

In `services/core/internal/infra/postgres/user_repository.go`, append after `CreateUser`:
```go
// EnsureUser idempotently mirrors a Keycloak-authenticated identity into
// Postgres. Unlike CreateUser (which also creates the Keycloak account),
// this assumes the Keycloak user already exists - it only needs to make
// sure the local profile row does too. ON CONFLICT DO NOTHING makes this
// safe to call on every authenticated request without a separate
// existence check.
func (c *Client) EnsureUser(ctx context.Context, organizationId int, uuid, username, firstName, lastName, email string) error {
	query := `
		INSERT INTO users (username, first_name, last_name, email, uuid, organization_id)
		VALUES (@username, @firstname, @lastname, @email, @uuid, @organizationId)
		ON CONFLICT (uuid) DO NOTHING`

	args := pgx.NamedArgs{
		"organizationId": organizationId,
		"uuid":           uuid,
		"username":       username,
		"firstname":      firstName,
		"lastname":       lastName,
		"email":          email,
	}

	_, err := c.db.Exec(ctx, query, args)
	if err != nil {
		return NewDatabaseError("error ensuring user", err)
	}

	return nil
}
```

- [ ] **Step 6: Regenerate mocks**

```bash
cd services/core && mockery
```
Expected: `services/core/internal/domain/user/mocks/user_repository_mock.go` now has an `EnsureUser` method. Confirm:
```bash
grep -c "EnsureUser" internal/domain/user/mocks/user_repository_mock.go
```
Expected: non-zero.

- [ ] **Step 7: Run test to verify it passes**

Run: `go test ./services/core/internal/domain/user/... -short -run TestUserService -v`
Expected: `PASS`, including the two new `EnsureProvisioned` subtests and every pre-existing subtest still green.

- [ ] **Step 8: Commit**

```bash
cd /Users/antoniomartinezlopez/dev/planeo
git add services/core/internal/domain/user/ services/core/internal/infra/postgres/user_repository.go
git commit -m "feat(core): add user.Service.EnsureProvisioned"
```

---

### Task 5: REST middleware — wire provisioning into the request pipeline

**Files:**
- Create: `services/core/internal/infra/rest/user_provisioning.go`
- Modify: `services/core/internal/infra/rest/server.go`

**Interfaces:**
- Consumes: `organization.Service.GetOrganizationByIAMID` (Task 3), `user.Service.EnsureProvisioned` (Task 4), `middlewares.OauthAccessClaims.{GivenName,FamilyName,PreferredUsername,Sub,Email,Groups}` (Task 2), `middlewares.AccessClaimsContextKey` (pre-existing, `libs/middlewares/claims.go:42`), `rest.Middleware` type alias (pre-existing, `server.go:48`).
- Produces: `rest.ProvisionUserMiddleware(userService user.Service, organizationService organization.Service) Middleware`, registered in `appMiddlewares`.

No dedicated unit test for this file: this repo's existing middleware (`AuthMiddleware`, `OrganizationCheckMiddleware`) has no isolated unit tests either — coverage for middleware in this codebase comes from HTTP-level integration tests, which Task 6 provides for this one.

- [ ] **Step 1: Create the middleware**

Create `services/core/internal/infra/rest/user_provisioning.go`:
```go
package rest

import (
	"strings"

	"planeo/libs/logger"
	"planeo/libs/middlewares"
	"planeo/services/core/internal/domain/organization"
	"planeo/services/core/internal/domain/user"

	"github.com/danielgtaylor/huma/v2"
)

// ProvisionUserMiddleware ensures the authenticated caller has a matching
// Postgres users row before the request reaches its handler. Keycloak is
// the source of truth for identity; this mirrors that identity into
// Postgres lazily, on whichever authenticated request happens to arrive
// first for a given sub, instead of requiring the two systems' ids to be
// pre-synced out of band (see
// docs/superpowers/specs/2026-07-20-jit-user-provisioning-design.md).
//
// This is best-effort: any failure here is logged and the request proceeds
// regardless. Most routes never touch the users table, and a transient
// failure self-heals on the next authenticated request since provisioning
// is attempted on every one.
func ProvisionUserMiddleware(userService user.Service, organizationService organization.Service) Middleware {
	return func(ctx huma.Context, next func(huma.Context)) {
		accessClaims, assertionCorrect := ctx.Context().Value(middlewares.AccessClaimsContextKey{}).(*middlewares.OauthAccessClaims)

		if !assertionCorrect || len(accessClaims.Groups) == 0 {
			next(ctx)
			return
		}

		log := logger.FromContext(ctx.Context())
		iamOrganizationID := strings.TrimPrefix(accessClaims.Groups[0], "/")

		org, err := organizationService.GetOrganizationByIAMID(ctx.Context(), iamOrganizationID)
		if err != nil {
			log.Error().Err(err).Str("iamOrganizationId", iamOrganizationID).Msg("failed to resolve organization for user provisioning")
			next(ctx)
			return
		}

		err = userService.EnsureProvisioned(
			ctx.Context(),
			org.Id,
			accessClaims.Sub,
			accessClaims.PreferredUsername,
			accessClaims.GivenName,
			accessClaims.FamilyName,
			accessClaims.Email,
		)
		if err != nil {
			log.Error().Err(err).Str("sub", accessClaims.Sub).Msg("failed to provision user")
		}

		next(ctx)
	}
}
```

- [ ] **Step 2: Register the middleware**

In `services/core/internal/infra/rest/server.go`, replace:
```go
	appMiddlewares := []Middleware{
		middlewares.AuthMiddleware(api, jwksURL, config.OauthIssuerUrl),
		middlewares.OrganizationCheckMiddleware(api, func(organizationId string) (string, error) {
```
with:
```go
	appMiddlewares := []Middleware{
		middlewares.AuthMiddleware(api, jwksURL, config.OauthIssuerUrl),
		ProvisionUserMiddleware(services.UserService, services.OrganizationService),
		middlewares.OrganizationCheckMiddleware(api, func(organizationId string) (string, error) {
```
(Only the two-line insertion — everything else in `appMiddlewares` and the rest of `InitRoutes` is unchanged.)

- [ ] **Step 3: Verify it builds**

```bash
cd /Users/antoniomartinezlopez/dev/planeo
go build ./...
```
Expected: exits 0, no errors.

- [ ] **Step 4: Commit**

```bash
git add services/core/internal/infra/rest/user_provisioning.go services/core/internal/infra/rest/server.go
git commit -m "feat(core): wire JIT user provisioning into the REST middleware chain"
```

---

### Task 6: Remove the hardcoded seed users, prove JIT works end-to-end

**Files:**
- Modify: `services/core/internal/infra/postgres/migrations/20241101135140_initialize_database.sql`
- Modify: `services/core/internal/test/organization/organization_test.go`

**Interfaces:**
- Consumes: Task 5's fully-wired middleware.

- [ ] **Step 1: Remove the hardcoded user seed rows**

In `services/core/internal/infra/postgres/migrations/20241101135140_initialize_database.sql`, replace:
```sql
INSERT INTO requests (text, subject, name, email, address, telephone, category_id, organization_id, reference_id, raw) VALUES
('Install new electrical outlets in the conference room', 'Installation electrics in conference room', 'Emily Clark', 'emily.clark@example.com', '123 Main St, Springfield', '555-1234', 1, 1, '1234-1', ''),
('Routine maintenance of the electrical wiring in the main office', 'Request: Maintenance electrical wiring' ,'Michael Scott', 'michael.scott@example.com', '456 Elm St, Scranton', '555-5678', 2, 1, '1234-2', ''),
('Repair the broken light fixtures in the hallway', 'Request for fixing broken light fixtures in hallway' ,'Sarah Lee', 'sarah.lee@example.com', '789 Oak St, Metropolis', '555-8765', 3, 1, '1234-3', ''),
('Order new circuit breakers for the electrical panel', 'Order: Circuit breakers No.PW-44021' ,'David Wilson', 'david.wilson@example.com', '101 Pine St, Gotham', '555-4321', 4, 1, '1234-4', ''),
('Customer support for troubleshooting a power outage issue', 'Customer support needed for outage problem' ,'Laura Martinez', 'laura.martinez@example.com', '202 Maple St, Star City', '555-6789', 5, 1, '1234-5', '');

INSERT INTO "users" ("username", "first_name", "last_name", "email", "uuid", "organization_id") VALUES 
('admin', 'admin', 'admin', 'admin@local.de', '7c806e52-e7cc-484b-843b-1242046590dc', 1),
('planner', 'planner', 'planner', 'planner@local.de', '146b3857-090e-453d-b1e6-8cdfbb1a6dcb', 1),
('user', 'User', 'User', 'user@local.de', 'd7eddb93-254e-4482-9a53-f31a5975dd1d', 1);
-- +goose StatementEnd
```
with:
```sql
INSERT INTO requests (text, subject, name, email, address, telephone, category_id, organization_id, reference_id, raw) VALUES
('Install new electrical outlets in the conference room', 'Installation electrics in conference room', 'Emily Clark', 'emily.clark@example.com', '123 Main St, Springfield', '555-1234', 1, 1, '1234-1', ''),
('Routine maintenance of the electrical wiring in the main office', 'Request: Maintenance electrical wiring' ,'Michael Scott', 'michael.scott@example.com', '456 Elm St, Scranton', '555-5678', 2, 1, '1234-2', ''),
('Repair the broken light fixtures in the hallway', 'Request for fixing broken light fixtures in hallway' ,'Sarah Lee', 'sarah.lee@example.com', '789 Oak St, Metropolis', '555-8765', 3, 1, '1234-3', ''),
('Order new circuit breakers for the electrical panel', 'Order: Circuit breakers No.PW-44021' ,'David Wilson', 'david.wilson@example.com', '101 Pine St, Gotham', '555-4321', 4, 1, '1234-4', ''),
('Customer support for troubleshooting a power outage issue', 'Customer support needed for outage problem' ,'Laura Martinez', 'laura.martinez@example.com', '202 Maple St, Star City', '555-6789', 5, 1, '1234-5', '');
-- +goose StatementEnd
```
(Note: the `organizations` seed insert a few lines above this block is untouched — only the `users` INSERT is removed. `users` no longer needs seeding at all; JIT provisioning creates rows on first authenticated request.)

- [ ] **Step 2: Add the explicit integration test proving JIT provisioning**

In `services/core/internal/test/organization/organization_test.go`, change the import block from:
```go
import (
	"fmt"
	"testing"

	jsonHelper "planeo/libs/json"
	"planeo/services/core/internal/domain/organization"
	"planeo/services/core/internal/test/utils"

	"github.com/stretchr/testify/assert"
)
```
to:
```go
import (
	"context"
	"fmt"
	"testing"

	jsonHelper "planeo/libs/json"
	"planeo/services/core/internal/domain/organization"
	"planeo/services/core/internal/domain/user"
	"planeo/services/core/internal/test/utils"

	"github.com/stretchr/testify/assert"
)
```

Then append this new top-level test function at the end of the file (after the closing `}` of `TestOrganizationIntegration`):
```go

func TestUserProvisioningIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	env := utils.NewIntegrationTestEnvironment(t)
	testApi := env.Api

	t.Run("creates a Postgres users row for a sub with no prior row on first authenticated request", func(t *testing.T) {
		session, err := env.GetUserSession("planner", "planner")
		assert.NoError(t, err)
		assert.NotNil(t, session)

		response := testApi.Get("/v1/organizations", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken))
		assert.Equal(t, 200, response.Code)

		users, err := env.DB.GetUsers(context.Background(), 1)
		assert.NoError(t, err)

		var provisioned *user.User
		for i := range users {
			if users[i].Email == "planner@local.de" {
				provisioned = &users[i]
			}
		}

		if assert.NotNil(t, provisioned, "expected a Postgres users row for planner@local.de to have been created") {
			assert.NotEmpty(t, provisioned.UUID)
			assert.Equal(t, "planner", provisioned.Username)
			assert.Equal(t, "planner", provisioned.FirstName)
		}
	})
}
```

This uses a fresh `NewIntegrationTestEnvironment` (its own testcontainers, its own empty `users` table) so "no prior row" is guaranteed rather than depending on sibling subtests' execution order.

- [ ] **Step 3: Run the full core integration suite**

```bash
task test:core:integration
```
Expected: all suites pass, including `TestOrganizationIntegration` (whose existing subtests already assert `GET /v1/organizations` returns data for admin/planner/user — this is the regression this whole plan fixes) and the new `TestUserProvisioningIntegration`.

- [ ] **Step 4: Commit**

```bash
git add services/core/internal/infra/postgres/migrations/20241101135140_initialize_database.sql services/core/internal/test/organization/organization_test.go
git commit -m "feat(core): remove hardcoded user seed rows, rely on JIT provisioning"
```

---

### Task 7: Retire the sync-script workaround, full workspace verification

**Files:**
- Delete: `dev/sync_dev_user_ids.sh`
- Delete: `infra/environments/local/keycloak/outputs.tf`
- Modify: `Taskfile.yml`
- Modify: `services/core/internal/test/testdata/realm-users.json`

**Interfaces:** None (this task only removes the now-unnecessary workaround built earlier in this session while diagnosing the bug this plan fixes; it doesn't touch any interface from Tasks 1-6).

- [ ] **Step 1: Delete the sync script and its Taskfile wiring**

```bash
git rm dev/sync_dev_user_ids.sh
```

In `Taskfile.yml`, remove this task block entirely:
```yaml
  infra:sync-dev-user-ids:
    desc: Copy Keycloak's live dev-user ids into Postgres's seeded users table
    cmds:
      - ./dev/sync_dev_user_ids.sh
```

In `Taskfile.yml`'s `up:` task, replace:
```yaml
  up:
    desc: Start Docker services and run DB migrations
    cmds:
      - echo "Starting up the dev environment..."
      - cd dev && ./start.sh
      - task: migrate:core
      - task: migrate:email
      - task: infra:sync-dev-user-ids
```
with:
```yaml
  up:
    desc: Start Docker services and run DB migrations
    cmds:
      - echo "Starting up the dev environment..."
      - cd dev && ./start.sh
      - task: migrate:core
      - task: migrate:email
```

- [ ] **Step 2: Delete the now-unused Terraform output**

```bash
git rm infra/environments/local/keycloak/outputs.tf
```

- [ ] **Step 3: Remove the pinned test-fixture user ids**

In `services/core/internal/test/testdata/realm-users.json`, remove all three `"id": "..."` lines (one per user object — `"id": "7c806e52-e7cc-484b-843b-1242046590dc"`, `"id": "146b3857-090e-453d-b1e6-8cdfbb1a6dcb"`, `"id": "d7eddb93-254e-4482-9a53-f31a5975dd1d"`), leaving every other field in each of the three user objects unchanged. Keycloak's realm-import will now assign each test user a random id, exactly like the live dev environment does — nothing needs it to match a hardcoded Postgres value anymore.

- [ ] **Step 4: Regenerate the test realm fixture and re-run the full core test suite**

```bash
task infra:sync-test-realm
task test:core:unit
task test:core:integration
```
Expected: both green. This is the proof that the fixture-side workaround from earlier in this session is genuinely no longer needed — the test users now get fully random Keycloak ids, and JIT provisioning still makes everything work.

- [ ] **Step 5: Full dev-environment fresh-cycle verification**

```bash
task down
task up
```
Expected: no errors; Keycloak/Postgres/Kafka come up clean; `tofu apply` succeeds; migrations run (no user seed step anymore).

Verify live:
```bash
TOKEN=$(curl -s --request POST --url http://localhost:8080/realms/local/protocol/openid-connect/token \
  --header 'content-type: application/x-www-form-urlencoded' \
  --data grant_type=password --data username=admin --data password=admin \
  --data scope="openid profile email" --data client_id=local --data client_secret=t4VlYX9CJIN3VTrlb5nRMXT8Qjr9SBdu \
  | python3 -c "import json,sys;print(json.load(sys.stdin)['access_token'])")
cd services/core && air -c air.toml &
sleep 8
curl -s -H "Authorization: Bearer $TOKEN" http://localhost:8000/api/v1/organizations
```
Expected: a JSON array containing the `local` organization (HTTP 200, non-empty) — with **no** `dev/sync_dev_user_ids.sh` step having run. Stop the core service afterward (`pkill -f "air -c air.toml"`).

- [ ] **Step 6: Full workspace lint/build verification**

```bash
cd /Users/antoniomartinezlopez/dev/planeo
go build ./...
go vet ./...
gofmt -l .
```
Expected: `go build`/`go vet` exit 0; `gofmt -l .` lists only the pre-existing, already-known `services/email/internal/infra/rest/api/errors.go` drift (untouched by this plan) — no new files listed.

```bash
task test:all
```
Expected: all suites (core unit/integration, email unit/integration, libs unit/integration) pass.

- [ ] **Step 7: Commit**

```bash
git add Taskfile.yml services/core/internal/test/testdata/realm-users.json services/core/internal/test/testdata/realm.json
git commit -m "chore(infra): retire dev_user_ids sync workaround now that JIT provisioning covers it"
```

---

## Self-Review

**Spec coverage:** Architecture (5 additions) — Tasks 1-5. Data flow — Task 5's middleware, proven end-to-end by Task 6's integration test. Error handling (best-effort, non-blocking) — Task 5's `ProvisionUserMiddleware` always calls `next` regardless of resolution/provisioning outcome. Testing — unit tests in Tasks 2/3/4, integration test in Task 6. Cleanup — Task 7 removes `dev/sync_dev_user_ids.sh`, its Taskfile wiring, `outputs.tf`'s `dev_user_ids` output, and the pinned test-fixture ids, exactly as the spec's Cleanup section specifies.

**Placeholder scan:** No TBD/TODO; every step has complete, runnable code or exact commands with expected output.

**Type consistency:** `EnsureProvisioned(ctx, organizationId int, uuid, username, firstName, lastName, email string) error` has the identical signature everywhere it appears — `user.Service` interface (Task 4 Step 3), `service.go` implementation (Task 4 Step 4), the unit test mock expectations (Task 4 Step 1), and the middleware's call site (Task 5 Step 1). `GetOrganizationByIAMID(ctx, iamOrganizationID string) (Organization, error)` is likewise identical across `organization.Repository`/`Service` (Task 3 Step 3), the postgres implementation (Task 3 Step 5), the unit test (Task 3 Step 1), and the middleware (Task 5 Step 1). `OauthAccessClaims.{GivenName,FamilyName,PreferredUsername}` (Task 2) match the field names the middleware reads (Task 5 Step 1) exactly.
