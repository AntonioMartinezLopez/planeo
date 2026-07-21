# Just-In-Time User Provisioning (services/core)

## Purpose

`services/core`'s Postgres `users` table has a `uuid` column that every organization-membership lookup joins against the authenticated JWT's `sub` claim (`GetOrganizationsByUserSub`, `internal/infra/postgres/organziation_repository.go:29-48`). Historically this worked because the hand-maintained `auth/local/realm.json` pinned each of the 3 dev users' Keycloak IDs to the exact UUIDs the Postgres seed migration (`internal/infra/postgres/migrations/20241101135140_initialize_database.sql:104-106`) also hardcodes — a coincidence a human kept in sync by hand.

The OpenTofu migration of the local Keycloak setup (`docs/superpowers/plans/2026-07-16-keycloak-iac-local.md`) broke that coincidence: the Keycloak Terraform provider does not support pinning a `keycloak_user` resource's `id` (it is a Terraform-managed meta-attribute, confirmed unsettable via `tofu validate`), so Keycloak now assigns random UUIDs on user creation. Postgres's hardcoded seed rows no longer match, silently breaking `GET /v1/organizations` (returns an empty array, not an error) and anything else keyed by `users.uuid`.

This spec replaces the hardcoded-UUID coupling with just-in-time (JIT) provisioning: the first time any authenticated request arrives for a `sub` with no matching Postgres row, the app creates one automatically from JWT claims. This removes the coupling entirely rather than re-establishing it (e.g. via a sync script), and generalizes correctly to any future non-local environment, where nobody can hand-sync UUIDs between two independently-provisioned systems.

## Non-goals

- **Does not change how admins create new users.** `user.Service.CreateUser` (`internal/domain/user/service.go:43-64`) already creates the Keycloak user first and takes back Keycloak's own generated UUID for the Postgres insert — this path has no coupling problem today and is untouched.
- **Does not add a "get current user profile" API.** No new read endpoint is introduced; provisioning is a side effect of the existing request pipeline, not a new feature surface.
- **Does not handle multi-organization membership.** The design assumes (matching the current schema and every existing dev/test user) that a user's JWT `groups` claim yields at most one organization to provision into. A token with an empty `groups` claim is logged and skipped, not an error.
- **Does not touch `dev`/`stage`/`prod` environments.** Same local-only scope as the plan this follows from.

## Architecture

Five additions, each small and isolated:

1. **`libs/middlewares/claims.go`** — add `GivenName`, `FamilyName`, `PreferredUsername` string fields (JSON tags `given_name`, `family_name`, `preferred_username`) to `OauthAccessClaims`. Keycloak already sends these in every access token (verified live this session); the struct simply doesn't map them yet.
2. **`organization` domain** — add `Service.GetOrganizationByIAMId(ctx context.Context, iamOrganizationId string) (Organization, error)`, backed by a new `Repository` method querying `WHERE iam_organization_id = @iamOrganizationId`. Resolves "which Postgres organization does this JWT's group belong to."
3. **`user` domain** — add `Service.EnsureProvisioned(ctx context.Context, organizationId int, uuid, username, firstName, lastName, email string) error`, backed by a `Repository` method using `INSERT INTO users (...) VALUES (...) ON CONFLICT (uuid) DO NOTHING` — the same idempotent-insert idiom already used in `internal/infra/inbox/inbox_repository.go` and `services/email/internal/infra/postgres/mail_repository.go`. A single conflict-safe insert is both the existence check and the write; no separate `GetByUUID` lookup is needed.
4. **New middleware**, `services/core/internal/infra/rest/user_provisioning.go` (not `libs/middlewares`, since it must call the services/core-specific `user.Service` — a concrete domain dependency the shared library does not and should not know about, per this repo's hexagonal layering rules). Registered in `server.go`'s `appMiddlewares`, immediately after `middlewares.AuthMiddleware(...)`.
5. **Migration edit** — remove the 3 hardcoded `INSERT INTO "users"` rows from `20241101135140_initialize_database.sql:103-107`. The `organizations` seed row (matched by the string `iam_organization_id = 'local'`, unaffected by this problem) is untouched.

## Data flow

```
Request
  → AuthMiddleware validates JWT, stashes *OauthAccessClaims in context
  → [new] user-provisioning middleware:
      - reads claims from context
      - if Groups is empty: log, call next, done
      - org, err := OrganizationService.GetOrganizationByIAMId(ctx, strings.TrimPrefix(claims.Groups[0], "/"))
      - if err != nil: log, call next, done
      - err = UserService.EnsureProvisioned(ctx, org.Id, claims.Sub, claims.PreferredUsername, claims.GivenName, claims.FamilyName, claims.Email)
      - if err != nil: log, call next, done  (best-effort; never blocks the request)
      - call next
  → OrganizationCheckMiddleware, permission checks, handler — unchanged
```

Running before the handler (not after, and not only on org-scoped routes) means even a brand-new user's very first call to `GET /v1/organizations` — which has no `{organizationId}` in its path and so never passes through `OrganizationCheckMiddleware` — still works correctly, because the Postgres row already exists by the time that handler's query runs.

## Error handling

Provisioning is best-effort and never fails the request it runs alongside: an empty `groups` claim, an unresolvable organization, or a database error are all logged and swallowed, with `next` always called. Most routes never touch the `users` table at all; there's no reason for a provisioning hiccup on one request to take down unrelated functionality. Because provisioning is attempted on every authenticated request, a transient failure self-heals on the next one.

## Testing

- **Unit** (`internal/domain/organization/service_test.go`, `internal/domain/user/service_test.go`, mocked repositories): `GetOrganizationByIAMId` found/not-found; `EnsureProvisioned` new-row and already-exists-is-a-no-op cases, and a repository-error case.
- **Integration** (new case in `internal/test/organization/` or `internal/test/user/`): mint a JWT for a `sub` with no existing Postgres row, make one authenticated request, assert the `users` row now exists with the expected organization, then assert `GET /v1/organizations` returns it.
- The rest of the existing integration suite gets this as incidental regression coverage: every test already authenticates before asserting, and once the migration stops pre-seeding admin/planner/user, those requests are what provisions them.

## Cleanup

This work retires the workaround built earlier in the same session while investigating the underlying bug:

- Delete `dev/sync_dev_user_ids.sh`.
- Remove its Taskfile task (`infra:sync-dev-user-ids`) and the `task up` step that calls it.
- Remove the `dev_user_ids` output from `infra/environments/local/keycloak/outputs.tf` (added solely for that script).
- Remove the `id` overrides added to `services/core/internal/test/testdata/realm-users.json`'s three dev users — once Postgres self-provisions from whatever `sub` a token actually carries, nothing needs Keycloak's user ID to match a hardcoded value.

The unrelated fixture-sync tooling from the same investigation (`dev/sync_test_realm.sh`, `infra:sync-test-realm`, `services/core/internal/test/testdata/realm-users.json`'s existence, the Keycloak version bump to 26.1.0, the `groups`-claim fix, the `Default Policy`/`Default Permission`/redacted-secret handling in the sync script) is unaffected by this spec and is not part of this work.
