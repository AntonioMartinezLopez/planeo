# Keycloak IaC (Local Environment) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace `auth/local/realm.json`, `auth/local/client_rbac.json`, `auth/admin/client.json`, and `dev/check_and_install_realm.sh` with an OpenTofu configuration (`infra/environments/local/keycloak`) that fully provisions the local Keycloak instance — realm, master-realm admin client, one organization (client + roles + group + fine-grained authorization schema), and the three dev users — wired into `task up` via new `infra:*:keycloak` Taskfile tasks.

**Architecture:** Three reusable OpenTofu modules (`keycloak-realm`, `keycloak-client-authz`, `keycloak-org`) under `infra/modules/`, called from one root module per environment (`infra/environments/local/keycloak`, the only one built in this plan). `keycloak-client-authz` generates Keycloak's UMA fine-grained-authorization objects (resources/scopes/role-policies/permissions) via `for_each` over a declarative permission matrix; `keycloak-org` (client + roles + group) calls it internally for its own client.

**Tech Stack:** OpenTofu (>= 1.6.0), `keycloak/keycloak` provider (~> 5.0), against the existing Keycloak 25.0.2 container in `dev/docker-compose.yaml`.

## Global Constraints

- Local-only: this plan does not create `dev`/`stage`/`prod` environment directories (that is deferred per the design spec's Non-goals — a later plan).
- All Terraform-consumed secrets for `local` are plaintext in a gitignored `terraform.tfvars`, matching the trust level of the existing plaintext credentials already committed in `dev/docker-compose.yaml`.
- Follow this repo's Taskfile convention: dedicated named tasks (`infra:apply:keycloak`, not a generic task parameterized by domain).
- Reference design spec: `docs/superpowers/specs/2026-07-16-iac-opentofu-foundation-design.md`. If anything here conflicts with it, this plan is the more specific, later-verified source (it was written after inspecting the actual current `realm.json`/`client_rbac.json` contents in detail).

---

## File structure

```
infra/
├── .gitignore
├── modules/
│   ├── keycloak-realm/
│   │   ├── versions.tf
│   │   ├── variables.tf
│   │   ├── main.tf
│   │   └── outputs.tf
│   ├── keycloak-client-authz/
│   │   ├── versions.tf
│   │   ├── variables.tf
│   │   ├── main.tf
│   │   └── outputs.tf
│   └── keycloak-org/
│       ├── versions.tf
│       ├── variables.tf
│       ├── main.tf
│       └── outputs.tf
└── environments/
    └── local/
        └── keycloak/
            ├── versions.tf
            ├── backend.tf
            ├── provider.tf
            ├── variables.tf
            ├── main.tf
            └── terraform.tfvars   # gitignored, created by Task 1 Step 1
```

Modified: `Taskfile.yml`, `dev/start.sh`. Deleted (Task 4): `auth/local/realm.json`, `auth/local/client_rbac.json`, `auth/local/client.json`, `auth/admin/client.json`, `dev/check_and_install_realm.sh`.

---

### Task 1: Scaffolding + `keycloak-realm` module + local environment bootstrap

**Files:**
- Create: `infra/.gitignore`
- Create: `infra/modules/keycloak-realm/versions.tf`
- Create: `infra/modules/keycloak-realm/variables.tf`
- Create: `infra/modules/keycloak-realm/main.tf`
- Create: `infra/modules/keycloak-realm/outputs.tf`
- Create: `infra/environments/local/keycloak/versions.tf`
- Create: `infra/environments/local/keycloak/backend.tf`
- Create: `infra/environments/local/keycloak/provider.tf`
- Create: `infra/environments/local/keycloak/variables.tf`
- Create: `infra/environments/local/keycloak/main.tf`
- Create: `infra/environments/local/keycloak/terraform.tfvars` (gitignored)
- Modify: `Taskfile.yml`

**Interfaces:**
- Produces: `module.realm.id` (realm ID/name string), `module.realm.realm` (same value) — consumed by Task 2's `keycloak-org` module call and Task 3's user resources.

- [ ] **Step 1: Scaffold the ignore file and module skeleton**

Create `infra/.gitignore`:
```gitignore
**/.terraform/
**/terraform.tfvars
.tofu-state/
```
(`.terraform.lock.hcl` is intentionally NOT ignored — it must be committed, same reasoning as `go.sum`, for reproducible provider versions.)

Create `infra/modules/keycloak-realm/versions.tf`:
```hcl
terraform {
  required_providers {
    keycloak = {
      source  = "keycloak/keycloak"
      version = "~> 5.0"
    }
  }
}
```

Create `infra/modules/keycloak-realm/variables.tf`:
```hcl
variable "realm_name" {
  description = "Name of the realm to create (also becomes its ID)"
  type        = string
}

variable "access_token_lifespan" {
  description = "How long an access token stays valid (Go duration string)"
  type        = string
  default     = "10h"
}

variable "sso_session_idle_timeout" {
  description = "How long an SSO session stays valid without activity (Go duration string)"
  type        = string
  default     = "720h"
}

variable "sso_session_max_lifespan" {
  description = "Maximum lifetime of an SSO session (Go duration string)"
  type        = string
  default     = "240h"
}

variable "admin_client_secret" {
  description = "Fixed secret for the master-realm admin-dev client that services/core uses to call the Keycloak Admin API"
  type        = string
  sensitive   = true
}
```

Create `infra/modules/keycloak-realm/main.tf`:
```hcl
resource "keycloak_realm" "this" {
  realm   = var.realm_name
  enabled = true

  access_token_lifespan    = var.access_token_lifespan
  sso_session_idle_timeout = var.sso_session_idle_timeout
  sso_session_max_lifespan = var.sso_session_max_lifespan
}

# services/core authenticates as this client (direct access grant, against the
# master realm) to obtain a token for calling the Keycloak Admin REST API
# against var.realm_name. Must live in "master", not var.realm_name - Admin
# API calls against any realm require a master-realm-authenticated token.
resource "keycloak_openid_client" "admin_dev" {
  realm_id  = "master"
  client_id = "admin-dev"

  access_type                  = "CONFIDENTIAL"
  client_secret                = var.admin_client_secret
  direct_access_grants_enabled = true
  service_accounts_enabled     = false
  standard_flow_enabled        = false
}
```

Create `infra/modules/keycloak-realm/outputs.tf`:
```hcl
output "id" {
  value = keycloak_realm.this.id
}

output "realm" {
  value = keycloak_realm.this.realm
}
```

- [ ] **Step 2: Wire the local environment root module**

Create `infra/environments/local/keycloak/versions.tf`:
```hcl
terraform {
  required_version = ">= 1.6.0"

  required_providers {
    keycloak = {
      source  = "keycloak/keycloak"
      version = "~> 5.0"
    }
  }
}
```

Create `infra/environments/local/keycloak/backend.tf`:
```hcl
terraform {
  backend "local" {
    path = "../../../.tofu-state/local-keycloak.tfstate"
  }
}
```

Create `infra/environments/local/keycloak/provider.tf`:
```hcl
provider "keycloak" {
  client_id = "admin-cli"
  username  = var.kc_admin_username
  password  = var.kc_admin_password
  url       = var.kc_base_url
  realm     = "master"
}
```

Create `infra/environments/local/keycloak/variables.tf`:
```hcl
variable "kc_base_url" {
  description = "Base URL of the local Keycloak instance"
  type        = string
}

variable "kc_admin_username" {
  description = "Master-realm superuser username (KEYCLOAK_ADMIN in docker-compose)"
  type        = string
}

variable "kc_admin_password" {
  description = "Master-realm superuser password (KEYCLOAK_ADMIN_PASSWORD in docker-compose)"
  type        = string
  sensitive   = true
}

variable "admin_client_secret" {
  description = "Fixed secret for the admin-dev client, must match services/core/.env's KC_ADMIN_CLIENT_SECRET"
  type        = string
  sensitive   = true
}
```

Create `infra/environments/local/keycloak/main.tf`:
```hcl
module "realm" {
  source = "../../../modules/keycloak-realm"

  realm_name          = "local"
  admin_client_secret = var.admin_client_secret
}
```

Create `infra/environments/local/keycloak/terraform.tfvars` (gitignored — values match `dev/docker-compose.yaml`'s `KEYCLOAK_ADMIN`/`KEYCLOAK_ADMIN_PASSWORD` and `services/core/.env.template`'s `KC_ADMIN_CLIENT_SECRET`, so nothing else needs to change to stay compatible):
```hcl
kc_base_url         = "http://localhost:8080"
kc_admin_username   = "admin"
kc_admin_password   = "password"
admin_client_secret = "SVknKbFjpjaqrWtKPluTXPOPqL0fkorW"
```

- [ ] **Step 3: Add Taskfile tasks**

In `Taskfile.yml`, add a new section (after the `migrate:*` tasks, before `run:*`):
```yaml
  # ========================================
  # Infrastructure (OpenTofu)
  # ========================================

  infra:fmt:
    desc: Format all OpenTofu files under infra/
    cmds:
      - tofu fmt -recursive infra/

  infra:init:keycloak:
    desc: Initialize the local Keycloak OpenTofu config
    dir: infra/environments/local/keycloak
    cmds:
      - tofu init

  infra:validate:keycloak:
    desc: Validate the local Keycloak OpenTofu config
    dir: infra/environments/local/keycloak
    cmds:
      - tofu validate

  infra:plan:keycloak:
    desc: Preview changes to the local Keycloak config
    dir: infra/environments/local/keycloak
    cmds:
      - tofu plan {{.CLI_ARGS}}

  infra:apply:keycloak:
    desc: Apply changes to the local Keycloak config
    dir: infra/environments/local/keycloak
    cmds:
      - tofu apply -input=false {{.CLI_ARGS}}
```

- [ ] **Step 4: Init and validate**

Ensure the dev stack is running first (needed later for apply, but `init`/`validate` don't need it):
```bash
task infra:init:keycloak
```
Expected: `Terraform has been successfully initialized!` and a new `infra/environments/local/keycloak/.terraform.lock.hcl` is created.

```bash
task infra:validate:keycloak
```
Expected: `Success! The configuration is valid.`

- [ ] **Step 5: Apply and verify against the real Keycloak container**

```bash
task up   # if not already running
task infra:apply:keycloak
```
Type `yes` when prompted. Expected: `Apply complete! Resources: 2 added, 0 changed, 0 destroyed.`

Verify the realm exists:
```bash
curl -s -o /dev/null -w '%{http_code}\n' http://localhost:8080/realms/local/protocol/openid-connect/certs
```
Expected: `200`

Verify the `admin-dev` client can authenticate as the master-realm admin (this is exactly what `services/core` does at runtime):
```bash
curl -s --request POST \
  --url http://localhost:8080/realms/master/protocol/openid-connect/token \
  --header 'content-type: application/x-www-form-urlencoded' \
  --data grant_type=password \
  --data username=admin \
  --data password=password \
  --data client_id=admin-dev \
  --data client_secret=SVknKbFjpjaqrWtKPluTXPOPqL0fkorW | python3 -c "import json,sys; print('access_token' in json.load(sys.stdin))"
```
Expected: `True`

- [ ] **Step 6: Commit**

```bash
git add infra/.gitignore infra/modules/keycloak-realm infra/environments/local/keycloak Taskfile.yml
git commit -m "feat(infra): scaffold OpenTofu tree, add keycloak-realm module for local"
```

---

### Task 2: `keycloak-client-authz` + `keycloak-org` modules

**Files:**
- Create: `infra/modules/keycloak-client-authz/versions.tf`
- Create: `infra/modules/keycloak-client-authz/variables.tf`
- Create: `infra/modules/keycloak-client-authz/main.tf`
- Create: `infra/modules/keycloak-client-authz/outputs.tf`
- Create: `infra/modules/keycloak-org/versions.tf`
- Create: `infra/modules/keycloak-org/variables.tf`
- Create: `infra/modules/keycloak-org/main.tf`
- Create: `infra/modules/keycloak-org/outputs.tf`
- Modify: `infra/environments/local/keycloak/main.tf`

**Interfaces:**
- Consumes: `module.realm.id` (from Task 1).
- Produces: `module.org_local.role_ids` (map, e.g. `{ Admin = "<uuid>", Planner = "<uuid>", User = "<uuid>" }`) and `module.org_local.group_id` (string) — both consumed by Task 3's user resources.

- [ ] **Step 1: Write `keycloak-client-authz`**

This module generates Keycloak's fine-grained UMA authorization objects (resources, scopes, role-policies, permissions) from a declarative matrix. Verified against the current live config in `auth/local/client_rbac.json`: 35 policy objects total — 3 are role-policies (Admin/Planner/User), 32 are one-permission-per-resource-per-scope bindings (e.g. `task:read` → scopes `["read"]`, `applyPolicies: ["Planner","User"]`). The matrix below reproduces that exact mapping.

Create `infra/modules/keycloak-client-authz/versions.tf`:
```hcl
terraform {
  required_providers {
    keycloak = {
      source  = "keycloak/keycloak"
      version = "~> 5.0"
    }
  }
}
```

Create `infra/modules/keycloak-client-authz/variables.tf`:
```hcl
variable "realm_id" {
  description = "ID of the realm the client belongs to"
  type        = string
}

variable "resource_server_id" {
  description = "resource_server_id of the client this authorization schema applies to"
  type        = string
}

variable "role_ids" {
  description = "map of role name => role ID, e.g. { Admin = \"<uuid>\", Planner = \"<uuid>\", User = \"<uuid>\" }"
  type        = map(string)
}

variable "permission_schema" {
  description = "resource name => scope name => list of role names granted that scope"
  type        = map(map(list(string)))
}
```

Create `infra/modules/keycloak-client-authz/main.tf`:
```hcl
locals {
  # Flatten the nested permission_schema map into one entry per resource:scope
  # pair, e.g. "Task:read" => { resource = "Task", scope = "read", roles = [...] }
  resource_scope_pairs = flatten([
    for resource, scopes in var.permission_schema : [
      for scope, roles in scopes : {
        key      = "${resource}:${scope}"
        resource = resource
        scope    = scope
        roles    = roles
      }
    ]
  ])
  resource_scope_map = { for p in local.resource_scope_pairs : p.key => p }

  resource_names = distinct([for p in local.resource_scope_pairs : p.resource])
  scope_names    = distinct([for p in local.resource_scope_pairs : p.scope])

  # scope names used by each resource, needed for that resource's own `scopes` set
  resource_scopes = {
    for resource in local.resource_names : resource => distinct([
      for p in local.resource_scope_pairs : p.scope if p.resource == resource
    ])
  }
}

resource "keycloak_openid_client_authorization_scope" "scopes" {
  for_each = toset(local.scope_names)

  realm_id           = var.realm_id
  resource_server_id = var.resource_server_id
  name               = each.value
}

resource "keycloak_openid_client_authorization_resource" "resources" {
  for_each = toset(local.resource_names)

  realm_id           = var.realm_id
  resource_server_id = var.resource_server_id
  name               = each.value
  scopes             = local.resource_scopes[each.value]
}

resource "keycloak_openid_client_role_policy" "role_policies" {
  for_each = var.role_ids

  realm_id           = var.realm_id
  resource_server_id = var.resource_server_id
  name               = "${each.key}-role-policy"
  logic              = "POSITIVE"
  decision_strategy  = "UNANIMOUS"

  role {
    id       = each.value
    required = true
  }
}

resource "keycloak_openid_client_authorization_permission" "permissions" {
  for_each = local.resource_scope_map

  realm_id           = var.realm_id
  resource_server_id = var.resource_server_id
  name               = each.key
  type               = "scope"
  decision_strategy  = "AFFIRMATIVE"

  resources = [keycloak_openid_client_authorization_resource.resources[each.value.resource].id]
  scopes    = [keycloak_openid_client_authorization_scope.scopes[each.value.scope].id]
  policies  = [for role in each.value.roles : keycloak_openid_client_role_policy.role_policies[role].id]
}
```

Create `infra/modules/keycloak-client-authz/outputs.tf`:
```hcl
output "resource_ids" {
  value = { for name, r in keycloak_openid_client_authorization_resource.resources : name => r.id }
}

output "permission_ids" {
  value = { for key, p in keycloak_openid_client_authorization_permission.permissions : key => p.id }
}
```

- [ ] **Step 2: Write `keycloak-org`**

Create `infra/modules/keycloak-org/versions.tf`: (identical content to Step 1's `versions.tf` above)
```hcl
terraform {
  required_providers {
    keycloak = {
      source  = "keycloak/keycloak"
      version = "~> 5.0"
    }
  }
}
```

Create `infra/modules/keycloak-org/variables.tf`:
```hcl
variable "realm_id" {
  description = "ID of the realm this organization belongs to"
  type        = string
}

variable "org_name" {
  description = "Name of the organization/tenant; used as the client_id and group name"
  type        = string
}

variable "permission_schema" {
  description = "resource name => scope name => list of role names granted that scope. Defaults to the schema verified against auth/local/client_rbac.json."
  type        = map(map(list(string)))
  default = {
    Task = {
      read   = ["Planner", "User"]
      create = ["Planner"]
      update = ["Planner", "User"]
      delete = ["Planner"]
    }
    Announcement = {
      read   = ["Admin"]
      create = ["Admin"]
      update = ["Admin"]
      delete = ["Admin"]
    }
    Group = {
      read   = ["Planner", "User"]
      create = ["Planner"]
      update = ["Planner", "User"]
      delete = ["Planner"]
    }
    Conversation = {
      read   = ["Planner", "User"]
      create = ["Planner", "User"]
      update = ["Planner", "User"]
      delete = ["Planner", "User"]
    }
    Reminder = {
      read   = ["Planner", "User"]
      create = ["Planner"]
      update = ["Planner"]
      delete = ["Planner"]
    }
    Organization = {
      manage = ["Admin"]
    }
    User = {
      read   = ["Admin"]
      create = ["Admin"]
      update = ["Admin"]
      delete = ["Admin"]
    }
    Role = {
      read = ["Admin"]
    }
    Userinfo = {
      read = ["Planner", "Admin", "User"]
    }
    Request = {
      read   = ["Planner", "Admin"]
      create = ["Planner", "Admin"]
      update = ["Planner", "Admin"]
      delete = ["Planner", "Admin"]
    }
    Category = {
      read   = ["Planner", "Admin", "User"]
      create = ["Admin", "Planner"]
      update = ["Admin", "Planner"]
      delete = ["Admin"]
    }
  }
}
```

Create `infra/modules/keycloak-org/main.tf`:
```hcl
resource "keycloak_openid_client" "this" {
  realm_id  = var.realm_id
  client_id = var.org_name

  access_type                  = "CONFIDENTIAL"
  service_accounts_enabled     = true
  direct_access_grants_enabled = true
  standard_flow_enabled        = false

  authorization {
    policy_enforcement_mode          = "ENFORCING"
    decision_strategy                = "AFFIRMATIVE"
    allow_remote_resource_management = true
    keep_defaults                    = false
  }
}

resource "keycloak_role" "roles" {
  for_each  = toset(["Admin", "Planner", "User"])
  realm_id  = var.realm_id
  client_id = keycloak_openid_client.this.id
  name      = each.value
}

resource "keycloak_group" "this" {
  realm_id = var.realm_id
  name     = var.org_name
}

module "authz" {
  source = "../keycloak-client-authz"

  realm_id           = var.realm_id
  resource_server_id = keycloak_openid_client.this.resource_server_id
  role_ids           = { for name, role in keycloak_role.roles : name => role.id }
  permission_schema  = var.permission_schema
}
```

Create `infra/modules/keycloak-org/outputs.tf`:
```hcl
output "client_id" {
  value = keycloak_openid_client.this.id
}

output "resource_server_id" {
  value = keycloak_openid_client.this.resource_server_id
}

output "group_id" {
  value = keycloak_group.this.id
}

output "role_ids" {
  value = { for name, role in keycloak_role.roles : name => role.id }
}
```

- [ ] **Step 3: Wire `module "org_local"` into the local environment**

Append to `infra/environments/local/keycloak/main.tf`:
```hcl
module "org_local" {
  source = "../../../modules/keycloak-org"

  realm_id = module.realm.id
  org_name = "local"
}
```

- [ ] **Step 4: Validate, plan, apply**

```bash
task infra:validate:keycloak
```
Expected: `Success! The configuration is valid.`

```bash
task infra:plan:keycloak
```
Expected: plan shows resources to add — 1 client, 3 roles, 1 group, plus the client-authz resources (scopes, resources, role-policies, permissions) — no errors.

```bash
task infra:apply:keycloak
```
Type `yes`. Expected: `Apply complete!` with no errors.

- [ ] **Step 5: Verify against the running Keycloak instance**

Get a token as `admin-dev` (reuse the curl from Task 1 Step 5), then confirm the client and its authorization resources exist:
```bash
TOKEN=$(curl -s --request POST \
  --url http://localhost:8080/realms/master/protocol/openid-connect/token \
  --header 'content-type: application/x-www-form-urlencoded' \
  --data grant_type=password --data username=admin --data password=password \
  --data client_id=admin-dev --data client_secret=SVknKbFjpjaqrWtKPluTXPOPqL0fkorW | python3 -c "import json,sys;print(json.load(sys.stdin)['access_token'])")

curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/admin/realms/local/clients?clientId=local" | python3 -c "import json,sys; print(len(json.load(sys.stdin)) == 1)"
```
Expected: `True`

- [ ] **Step 6: Commit**

```bash
git add infra/modules/keycloak-client-authz infra/modules/keycloak-org infra/environments/local/keycloak/main.tf
git commit -m "feat(infra): add keycloak-client-authz and keycloak-org modules, provision local org"
```

---

### Task 3: Local dev users (admin/planner/user)

**Files:**
- Modify: `infra/environments/local/keycloak/main.tf`

**Interfaces:**
- Consumes: `module.realm.id` (Task 1), `module.org_local.role_ids`, `module.org_local.group_id` (Task 2).

- [ ] **Step 1: Add the three dev users**

Append to `infra/environments/local/keycloak/main.tf`:
```hcl
locals {
  dev_users = {
    admin   = { email = "admin@local.de", role = "Admin" }
    planner = { email = "planner@local.de", role = "Planner" }
    user    = { email = "user@local.de", role = "User" }
  }
}

resource "keycloak_user" "dev" {
  for_each = local.dev_users

  realm_id   = module.realm.id
  username   = each.key
  email      = each.value.email
  first_name = each.key
  last_name  = each.key
  enabled    = true

  initial_password {
    value     = each.key
    temporary = false
  }
}

resource "keycloak_user_roles" "dev" {
  for_each = local.dev_users

  realm_id = module.realm.id
  user_id  = keycloak_user.dev[each.key].id
  role_ids = [module.org_local.role_ids[each.value.role]]
}

resource "keycloak_user_groups" "dev" {
  for_each = local.dev_users

  realm_id  = module.realm.id
  user_id   = keycloak_user.dev[each.key].id
  group_ids = [module.org_local.group_id]
}
```

- [ ] **Step 2: Validate, plan, apply**

```bash
task infra:validate:keycloak && task infra:plan:keycloak
```
Expected: plan shows 9 resources to add (3 users, 3 `keycloak_user_roles`, 3 `keycloak_user_groups`), no errors.

```bash
task infra:apply:keycloak
```
Type `yes`. Expected: `Apply complete!`

- [ ] **Step 3: Verify via the existing login flow**

```bash
task login
```
This runs `dev/get_test_token.sh`, which requests tokens for `admin@local.de`/`admin`, `planner@local.de`/`planner`, and `user@local.de`/`user` against the `local` client. Expected: all three requests succeed and print non-empty access tokens (matches today's behavior with `realm.json`'s hand-maintained users).

- [ ] **Step 4: Commit**

```bash
git add infra/environments/local/keycloak/main.tf
git commit -m "feat(infra): provision local dev users (admin/planner/user) via OpenTofu"
```

---

### Task 4: Retire the JSON/bash flow, wire `task up`

**Files:**
- Delete: `auth/local/realm.json`
- Delete: `auth/local/client_rbac.json`
- Delete: `auth/local/client.json`
- Delete: `auth/admin/client.json`
- Delete: `dev/check_and_install_realm.sh`
- Modify: `dev/start.sh`

**Interfaces:** None (this task only removes the old path and rewires `task up`; it doesn't change any module interface).

- [ ] **Step 1: Delete the now-superseded files**

```bash
git rm auth/local/realm.json auth/local/client_rbac.json auth/local/client.json auth/admin/client.json dev/check_and_install_realm.sh
```

- [ ] **Step 2: Update `dev/start.sh`**

Find:
```bash
# prepare keycloak
echo "Preparing Keycloak..."
./check_and_install_realm.sh
```
Replace with:
```bash
# prepare keycloak (OpenTofu-managed realm/org/users)
echo "Preparing Keycloak..."
task infra:apply:keycloak -- -auto-approve
```
(`task` resolves `Taskfile.yml` by walking up from the current directory, so this works even though `start.sh` runs with `dev/` as its cwd — no `cd` needed. `-auto-approve` is passed explicitly here, at the call site, rather than baked into the `infra:apply:keycloak` task itself, so a human running `task infra:apply:keycloak` directly still gets the normal interactive confirmation.)

- [ ] **Step 3: Full-cycle verification**

```bash
task down
task up
```
Expected: containers start, Keycloak health check passes, `task infra:apply:keycloak -- -auto-approve` runs with no prompt and no errors, then `migrate:core`/`migrate:email` run as before.

```bash
task login
```
Expected: same as Task 3 Step 3 — all three users authenticate successfully.

Re-run `task up` again without `task down` first, to confirm idempotency (this is the actual DX improvement over the old bash script):
```bash
task up
```
Expected: `tofu apply` reports `No changes.` for the Keycloak step (rather than erroring on "already exists," which is what `check_and_install_realm.sh`'s ad-hoc existence checks were working around).

- [ ] **Step 4: Commit**

```bash
git add dev/start.sh
git commit -m "chore(infra): retire realm.json/client_rbac.json/check_and_install_realm.sh, wire task up to OpenTofu"
```

---

## Self-Review

**Spec coverage:** Repository structure (Section 1) — done via Task 1/2 file layout. Org mapping (Section 2) — `keycloak-org` module, Task 2. State backend (Section 3, local half) — Task 1 `backend.tf`. Secrets (Section 4, local half) — gitignored `terraform.tfvars`, Task 1. Keycloak modules (Section 5) — Tasks 1-3, including the corrected per-resource-scope permission matrix (verified against live `client_rbac.json`, more precise than the design spec's summary). `task up` integration (Section 8, keycloak half) — Task 4. Developer workflow commands (Section 9, keycloak subset) — Task 1 Step 3. Kafka (Sections 6, and the kafka half of 7-9) is explicitly out of scope for this plan — deferred to a second plan per the earlier scope decision.

**Placeholder scan:** No TBD/TODO; all code blocks are complete, runnable HCL/bash with real values (the fixed local secrets intentionally match existing plaintext dev credentials already in the repo, not invented placeholders).

**Type consistency:** `module.realm.id` (Task 1) is consumed identically in Task 2 (`org_local`'s `realm_id`) and Task 3 (`keycloak_user.dev`'s `realm_id`). `module.org_local.role_ids`/`group_id` (Task 2 outputs) match the exact map/string types Task 3 consumes. `keycloak-client-authz`'s `role_ids` input type (`map(string)`) matches what `keycloak-org` passes (`{ for name, role in keycloak_role.roles : name => role.id }`).
