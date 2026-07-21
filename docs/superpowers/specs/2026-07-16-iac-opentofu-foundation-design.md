# IaC Foundation for Keycloak & Kafka Configuration (OpenTofu)

## Purpose

Keycloak realm/client/authorization config is currently hand-maintained as large JSON exports (`auth/local/realm.json` — 1659 lines, `auth/local/client_rbac.json` — 683 lines) applied via a bespoke bash script (`dev/check_and_install_realm.sh`) that does its own existence checks before creating things. Kafka topics rely on `KAFKA_AUTO_CREATE_TOPICS_ENABLE=true`, so topic shape is an accidental side effect of whichever client first produces to a given name, not a deliberate, reviewable change.

This spec introduces a `infra/` OpenTofu tree that manages both as declarative, reusable, environment-scoped configuration: Keycloak realms/clients/groups/authorization-policies, and Kafka topics/ACLs/SCRAM credentials. It also establishes the environment, state-backend, and secrets conventions that `dev`/`stage`/`prod` will use once they exist, without requiring any cloud account or deployment decision today.

**Provider versions to pin at implementation time** (confirmed to exist during design, but not version-locked here): `keycloak/keycloak` (successor to the archived `mrparkers/keycloak` namespace; verify current release supports Keycloak 25.0.2, since the compose file pins that server version and Keycloak provider/server skew is a known source of breakage) and `Mongey/kafka` (topics, ACLs, SCRAM credentials).

## Non-goals

- **Does not provision the Keycloak/Kafka instances themselves** for `dev`/`stage`/`prod` (no VM, Kubernetes, or managed-service provisioning). This design only manages configuration against an admin endpoint that's assumed to already exist. Local is the exception: the local Kafka broker's SASL listener and bootstrap SCRAM credential are a one-time docker-compose change, described in Section 6, because "connect only with secrets" was explicitly pulled into scope for Kafka.
- **Does not pick a secrets manager** (1Password/Vault/Doppler/SOPS/etc.) for `dev`/`stage`/`prod`. Section 4 designs the *interface* (env vars in, nothing tool-specific in `.tf` or Taskfile) so that decision is fully deferred without blocking anything here.
- **Does not pick an S3-compatible storage provider** (AWS/MinIO/R2/Backblaze B2/etc.) for remote state. Section 3 designs the backend to be provider-agnostic; picking one is a `dev`-environment bring-up task, not part of this spec.
- **Does not provision real per-organization users.** Terraform-managed `keycloak_user` is local-dev-only (three throwaway accounts in the `local` org), matching the Keycloak provider maintainers' own guidance against using that resource in production. Real org users go through the app's own UI/API, not IaC.
- **Does not implement `dev`/`stage`/`prod` Keycloak/Kafka config.** Those environments get scaffolding (directory structure, module calls, backend files) so they slot in later with no rework, but are never `tofu init`'d or applied as part of this work. Only `local` is actually built and applied.

## 1. Repository structure

```
infra/
├── modules/
│   ├── keycloak-realm/       # realm + realm-wide settings + admin bootstrap client
│   ├── keycloak-client-authz/ # resources/scopes/role-policies/permissions for one client
│   ├── keycloak-org/         # one tenant: client + roles + group; calls keycloak-client-authz
│   ├── kafka-topics/         # topic + partitions + retention, for_each over a map
│   └── kafka-scram-user/     # one service credential + its scoped ACLs
├── environments/
│   ├── local/
│   │   ├── keycloak/          # main.tf, backend.tf (local backend), terraform.tfvars (gitignored)
│   │   └── kafka/
│   ├── dev/       # same shape as local, backend.tf uses "s3"; not applied
│   ├── stage/     # same shape as dev; not applied
│   └── prod/      # same shape as dev; not applied
└── README.md
```

Each `environments/<env>/<domain>/` directory is a fully standalone OpenTofu root module: its own backend, its own state, its own apply, calling the shared modules with environment-specific inputs. This directory-per-environment shape (rather than OpenTofu workspaces) is deliberate: workspaces share one configured backend, which cannot represent "local uses a local-file backend, dev/stage/prod use S3" — that split is exactly what's needed here.

## 2. Organization mapping (confirmed against current config)

**Correction (2026-07-21):** the original design below (one client per organization) was found to be architecturally unworkable once the application code was audited: role management (`services/core/internal/infra/keycloak/service.go`) and permission checks (`libs/middlewares/permission_validation.go`'s `PermissionMiddlewareConfig`) both resolve against a single Keycloak client fixed at process startup, with no per-request or per-org override. A second org's client could never be reached by this code, and per-org login clients create a chicken-and-egg problem for the web app's single OIDC login flow (you'd need to know the user's org to pick its client before authenticating them). Per-org clients also didn't buy the original goal — someone still has to centrally define each org's roles/permissions in Keycloak either way, so the model added client-management overhead without enabling org-defined permissions.

The corrected model: one realm per environment (e.g. realm `local`, unchanged), with **one shared client per environment** (`modules/keycloak-client`) that every organization authenticates against, and **one Keycloak group per organization** (`modules/keycloak-org`, now group-only) for tenant identity — which is how organization resolution already worked in practice (`ProvisionUserMiddleware` reads the JWT `groups` claim). Onboarding a new org becomes one `keycloak-org` module call (a group), not a new client. Per-org customizable permissions, if ever needed, belong at the application layer (Postgres-backed permission composition), not as per-org Keycloak clients — see `docs/superpowers/specs/` for that follow-up if/when it becomes a roadmap item.

## 3. Environment & state backend strategy

Each `environments/<env>/<domain>/backend.tf` differs by environment:

**`local`** — the `local` backend, a plain file on disk:
```hcl
terraform {
  backend "local" {
    path = "../../../.tofu-state/local-keycloak.tfstate"
  }
}
```
Gitignored. No account or server needed, matching how `dev/.env` already works.

**`dev` / `stage` / `prod`** — the `s3` backend, written to stay object-store-agnostic rather than AWS-specific:
```hcl
terraform {
  backend "s3" {
    bucket       = "planeo-tofu-state"   # supplied via -backend-config, not hardcoded
    key          = "dev/keycloak.tfstate"
    region       = "auto"                 # or a real AWS region
    endpoints    = { s3 = "https://..." } # override for MinIO/R2/B2; omit for AWS
    use_lockfile = true                   # native conditional-write locking (OpenTofu 1.10+), no DynamoDB
  }
}
```
Because this only needs an S3-*protocol* endpoint, the same backend config works against real AWS S3, self-hosted MinIO, Cloudflare R2, or Backblaze B2 — only the `-backend-config` values (bucket/endpoint/credentials) change per environment, supplied at `tofu init` time, never committed. `use_lockfile` replaces the older DynamoDB-table-for-locking pattern with a conditional-write lock object in the same bucket.

Since no bucket exists yet, `dev/stage/prod` backends are written but `tofu init` for them is simply never run until a provider is chosen — nothing here depends on that decision existing today.

**State sensitivity**: Terraform state stores all resource attributes — including secrets like a Kafka SCRAM password or a Keycloak client secret — in plaintext JSON. Whichever bucket eventually backs `dev/stage/prod` state must have encryption-at-rest and IAM/access-policy scoped narrowly to that bucket/prefix; this is not optional once those environments become real.

### Example: AWS-specific auth/state flow (for reference, when that decision is made)

One-time bootstrap (manual, or a small separate bootstrap config applied once with an admin account):
1. Create the bucket — versioning **on** (undo button for state corruption), encryption **on**.
2. Create an IAM policy scoped to that bucket/prefix only (`GetObject`/`PutObject`/`DeleteObject`/`ListBucket`), not account-wide S3 access.

Local developer flow:
1. Authenticate to AWS normally (`aws configure` or SSO) — credentials land in `~/.aws/credentials` or as `AWS_ACCESS_KEY_ID`/`AWS_SECRET_ACCESS_KEY` env vars.
2. `tofu init` in the target environment/domain folder — the S3 backend picks up those credentials via the standard AWS SDK credential chain and reads/writes the state object over the S3 API.
3. `tofu plan`/`apply` — before writing, attempts a conditional PUT of a lock object (`use_lockfile`); a concurrent `apply` fails fast with "state locked" instead of corrupting the file.

CI (GitHub Actions), if reached later:
1. OIDC federation — an IAM role trusts GitHub's OIDC provider for this specific repo; no long-lived secret stored in GitHub.
2. `aws-actions/configure-aws-credentials` (with `permissions: id-token: write`) exchanges GitHub's OIDC token for short-lived STS credentials for the job.
3. Same `tofu init/plan/apply` steps pick those up transparently.

The backend's auth is entirely separate from the Keycloak/Kafka *provider* auth (Section 4) — this is only about who can read/write the state file.

## 4. Secrets strategy

Two distinct flows, kept conceptually separate:

**A. Secrets Terraform *consumes*** (state-bucket credentials, Keycloak admin password to authenticate the provider itself):
- `local`: plaintext, gitignored `terraform.tfvars`, same trust level as the existing plaintext creds in `dev/docker-compose.yaml`.
- `dev`/`stage`/`prod`: no tool chosen now. The `.tf` files and Taskfile only ever expect these to already be present as environment variables (`TF_VAR_*`) by the time `tofu` runs — *how* they get set is swappable and decided later (`op run -- ...` for 1Password, `doppler run -- ...`, Vault, SOPS, or anything else). Nothing in this repo's code references a specific secrets tool, so picking one later is a zero-rework decision.

**B. Secrets Terraform *generates*** (a Kafka SCRAM password, a Keycloak client secret for a new org's client): the reverse flow. Terraform creates the value (`random_password` resource, or Keycloak/Kafka generates and returns it) as a `sensitive` output. A Task target reads `tofu output -json` and, for now, writes it to a gitignored `.env` for local service consumption. Pushing generated secrets into a real secrets manager for `dev/stage/prod` is a follow-up, not decided here.

Either way, the state-sensitivity caveat from Section 3 applies: these values also live in plaintext inside the state file regardless of which flow produced them.

## 5. Keycloak module design

**`modules/keycloak-realm`** — one call per environment: the realm itself, realm-wide session/security settings (token lifespans, brute-force protection — currently copied verbatim across environments in `realm.json`), and the bootstrap admin client (today's `auth/admin/client.json`).

**`modules/keycloak-client-authz`** — takes a `resource_server_id` (a client) and a `permission_schema` variable, and generates the fine-grained UMA authorization config via `for_each`: resources, scopes, role-based policies, and permissions. Confirmed against `auth/local/client_rbac.json`: this shape is **identical across all 12 resource types (Task, Announcement, Group, Conversation, Reminder, Organization, User, Role, Userinfo, Request, Category) × 4 scopes (read/create/update/delete) × 3 role policies (Admin/Planner/User)**, and repeats identically per client — one static permission schema, replicated per client only because Keycloak scopes authorization services per-client. `permission_schema` defaults to this shape so it isn't repeated per org, but stays overridable. Verified the Keycloak provider supports this via `keycloak_authorization_resource`/`_policy`/`_permission` resources tied to a `resource_server_id`.

**`modules/keycloak-client`** — one call per environment's realm: the shared OIDC client every organization in that realm authenticates against (confidential, service accounts on, direct grants on, standard flow on for browser login), client roles (`Admin`, `Planner`, `User`), and an internal call to `keycloak-client-authz` for that client's authorization schema. See the Section 2 correction — this replaces what was originally `modules/keycloak-org`'s client-provisioning responsibility.

**`modules/keycloak-org`** — one call per tenant/organization within an environment's realm: just a group at `/<org_name>`, used for tenant identity/membership (JWT `groups` claim) against the shared client above. No client, roles, or authorization config of its own.

**Local-only dev users** — not a module (only ever used once, for the `local` org; no reuse to abstract for). Declared directly in `environments/local/keycloak/main.tf`:
```hcl
resource "keycloak_user" "admin" {
  realm_id = module.realm.id
  username = "admin"
  email    = "admin@local.de"
  initial_password {
    value     = "admin"
    temporary = false   # stable password, matches today's non-expiring dev creds
  }
}
```
(same pattern for `planner`/`planner@local.de` and `user`/`user@local.de`). Role assignment references `module.org_local`'s client roles/group. Note: the provider's own docs describe `keycloak_user` as not intended for production use ("prefer federating users from an external source") — acceptable here because these are local-dev-only, already-public throwaway credentials; real org users are out of scope per the Non-goals section.

## 6. Kafka module design

**Broker change (one-time, docker-compose, the "minimal auth into scope" piece)**: add a `SASL_PLAINTEXT` listener alongside the existing `PLAINTEXT_HOST` one, and seed exactly one bootstrap SCRAM admin at container init via `kafka-storage.sh format --add-scram 'SCRAM-SHA-256=[name=admin,password=...]'` (confirmed this must happen at KRaft cluster-formatting time, before the broker starts — the Terraform Kafka provider itself needs an authenticated admin connection, so it cannot be the thing that creates the first credential). Password sourced from the same local plaintext secrets file as everything else local. This exactly mirrors the Keycloak bootstrap-admin-client pattern in Section 5. Everything else (per-service credentials, ACLs) is created by Terraform authenticating as that bootstrap admin.

**`modules/kafka-topics`** — `for_each` over a `topics` map:
```hcl
topics = {
  "email.received" = { partitions = 3, retention_ms = 604800000 }
}
```
Replaces reliance on `KAFKA_AUTO_CREATE_TOPICS_ENABLE` (`"true"` today) — real environments should have this off, so topic shape is only ever a deliberate Terraform change.

**`modules/kafka-scram-user`** — one call per Kafka client *service*, not per organization (Kafka access is scoped to this app's own services, not tenants). Confirmed the current Kafka clients are two dedicated sidecar binaries introduced by the NATS→Kafka migration (merged in `main` as of commit `f9fafa9`, PR #48): `services/email`'s `email-received-producer` and `services/core`'s `email-received-consumer`. Each module call takes a username and a list of `{ topic, operations }` grants, and creates a generated password (`random_password`, unless supplied) plus the SCRAM credential, and narrowly-scoped `kafka_acl` resources — e.g. `email-received-producer` → `Write` only on `email.received`; `email-received-consumer` → `Read`+`Describe` on `email.received` and its own consumer group only.

## 7. Rollout plan

Build and apply `local` completely first — this is the part that replaces `auth/local/realm.json`, `auth/local/client_rbac.json`, `auth/admin/client.json`, and `dev/check_and_install_realm.sh`, plus the Kafka SASL/topics/SCRAM setup on top of the now-merged Kafka migration. `dev/stage/prod` get the directory structure, backend files, and module calls written per the Non-goals section, but are never `tofu init`'d or applied as part of this work.

## 8. `task up` integration

`tofu apply` is naturally idempotent (diffs against real state before changing anything), which replaces the custom "does this already exist" checks in `check_and_install_realm.sh` outright rather than just relocating them. Proposed change to `dev/start.sh`:

1. `docker compose up` (unchanged)
2. wait for Keycloak health (unchanged)
3. **wait for Kafka health** (new — needed before Terraform can authenticate with the bootstrap SCRAM admin from Section 6; currently missing)
4. `task infra:apply:keycloak -- -auto-approve`
5. `task infra:apply:kafka -- -auto-approve`
6. existing `migrate:core` / `migrate:email` continue after, unchanged

This deletes `check_and_install_realm.sh` and the `KAFKA_AUTO_CREATE_TOPICS_ENABLE` reliance entirely. Running `task up` again (e.g. after `task down && task up`) re-converges instead of erroring on "already exists."

**`-auto-approve` is opt-in per invocation, not tied to environment.** `start.sh` explicitly passes `-auto-approve` via `-- -auto-approve`; a human running `task infra:apply:keycloak` bare (even against `local`) gets the normal interactive confirmation. This is deliberately not a Taskfile-level "if env == local then auto-approve" conditional, since that would silently start auto-approving any time `INFRA_ENV` happened to be `local` for an unrelated reason.

## 9. Developer workflow (command reference)

Following this repo's existing Taskfile convention of dedicated named tasks (`migrate:core`, `run:core`, `test:core:unit`, `build:email:email-received-producer` — never a generic task parameterized by a component name), domain gets a dedicated task (structural difference: different modules/providers per domain), while environment is a variable (configuration difference: same task logic, different target directory) — following the `VERSION: '{{.VERSION | default "latest"}}'` pattern already present at the top of this Taskfile:

```yaml
vars:
  INFRA_ENV: '{{.INFRA_ENV | default "local"}}'

tasks:
  infra:apply:keycloak:
    dir: infra/environments/{{.INFRA_ENV}}/keycloak
    cmds: [tofu apply {{.CLI_ARGS}}]
  infra:apply:kafka:
    dir: infra/environments/{{.INFRA_ENV}}/kafka
    cmds: [tofu apply {{.CLI_ARGS}}]
  infra:apply:                     # aggregate, mirrors test:all/build:all/lint:all
    cmds: [{task: infra:apply:keycloak}, {task: infra:apply:kafka}]

  infra:plan:keycloak:
  infra:plan:kafka:
  infra:plan:                      # aggregate dry-run of both

  infra:init:keycloak:
  infra:init:kafka:
  infra:init:                      # aggregate

  infra:validate:keycloak:
  infra:validate:kafka:
  infra:fmt:                       # tofu fmt -recursive infra/ (env-agnostic, already global)
```

Usage:
```bash
task infra:apply:keycloak                    # applies against local (default)
INFRA_ENV=dev task infra:apply:keycloak       # same task, targets dev once it's real — no new task needed
task infra:apply:keycloak -- -auto-approve    # what start.sh calls; skips confirmation
```

When `dev`/`stage`/`prod` become real, this scales via the `INFRA_ENV` var with no new task definitions required — the `-backend-config` values for the non-local S3 backend are supplied the same way (either baked into a per-environment `-backend-config` file the task passes, or via additional env vars), decided at the point one of those environments is actually built.

What each command does:
- **`init`**: connects to the backend (state), downloads/pins providers (`.terraform.lock.hcl`), resolves module sources. No infrastructure touched, no Keycloak/Kafka API calls.
- **`validate`**: pure static check of the `.tf` files — no backend, no provider API calls. Cheap enough for a pre-commit or CI check.
- **`plan`**: reads current state, calls out to Keycloak/Kafka to see live state, diffs against desired config, shows what *would* change. No mutation.
- **`apply`**: re-runs plan, asks for confirmation (unless `-auto-approve` is passed), then calls the Keycloak/Kafka APIs to converge reality to the config, and writes new state.

Re-run `init` whenever a provider is added/changed, a module source changes, or the `backend` block changes; otherwise `plan`/`apply` reuse what `init` already set up.
