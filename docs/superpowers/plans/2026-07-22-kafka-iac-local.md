# Kafka IaC (Local Environment) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Bring the Kafka half of `docs/superpowers/specs/2026-07-16-iac-opentofu-foundation-design.md` to parity with the already-implemented Keycloak half — a SASL/SCRAM-authenticated local Kafka broker, with topics and per-service credentials declared in OpenTofu instead of relying on `KAFKA_AUTO_CREATE_TOPICS_ENABLE` and unauthenticated access.

**Architecture:** One-time `dev/docker-compose.yaml` change seeds a bootstrap SCRAM admin at KRaft cluster-formatting time (verified empirically against the real `apache/kafka` image — see Task 1). Two new OpenTofu modules (`kafka-topics`, `kafka-scram-user`) mirror the existing `keycloak-client-authz`/`keycloak-org` pattern: declarative topics, and one generated SCRAM credential + narrowly-scoped ACLs per Kafka client service (not per organization — Kafka access is scoped to this app's own services, exactly as the original spec's Section 6 already said). `dev/start.sh` gains a second apply phase (Kafka, after Keycloak) plus a secret-sync step, because — unlike Keycloak's pinned client secret — these credentials are Terraform-*generated* and don't exist until after the first `tofu apply`.

**Tech Stack:** OpenTofu, `Mongey/kafka` provider (Terraform Registry, latest `0.13.1` at time of writing — confirmed via registry API), `apache/kafka:4.3.1` Docker image (pinned; today's compose file uses the floating `apache/kafka:latest` tag, which resolved to this exact version during this plan's verification), `github.com/twmb/franz-go` v1.21.5 (already a direct dependency — `pkg/sasl/scram` subpackage confirmed present in the vendored module, no `go.mod` change needed).

## Global Constraints

- Local only. `dev`/`stage`/`prod` get no Kafka provisioning in this plan (matching the spec's own Non-goals for the Keycloak half — this is the equivalent scope boundary for Kafka).
- No secrets-manager decision. Secrets Terraform *consumes* (the bootstrap admin password) go in the existing plaintext, gitignored `dev/.env` / `terraform.tfvars` convention. Secrets Terraform *generates* (per-service SCRAM passwords) are synced via `tofu output -json` into `dev/.env`, per spec Section 4B — this plan adds the concrete sync mechanism, since the Keycloak half never needed one (its client secret is a fixed `var`, not generated).
- The existing topic name is `email-received` (hyphen), not `email.received` — the spec's Section 6 used a dot-separated example name; the actual constant is `libs/events/contracts/contracts.go:27`'s `EmailReceivedTopic = "email-received"`. Use the real name everywhere in this plan.
- The two Kafka-client sidecars (`email-received-producer`, `email-received-consumer`) never start with a blank/placeholder credential in the first place. Rather than relying on `franz-go`'s lazy connection behavior to self-heal a container started too early, Task 3 gates both services behind a Compose `profiles:` entry so the *first* `docker compose up` (no `--profile` flag) skips them entirely; they're started for the first time, with real credentials already in `dev/.env`, only after `task infra:apply:kafka` + `task infra:sync-secrets:kafka` have run. This avoids the whole class of "started with an empty credential, needs a recreate later" problems rather than working around it.
- Two independent listener needs, mirroring the existing `PLAINTEXT`/`PLAINTEXT_HOST` split already in `dev/docker-compose.yaml`: an internal SASL listener for the two sidecar containers (Docker network hostname `kafka`), and a host-reachable SASL listener for OpenTofu (which runs on the host, exactly like it already does for Keycloak against `localhost:8080`).

## File structure

```
dev/
├── docker-compose.yaml                          # modify: kafka service (SASL bootstrap), two sidecar services (SASL creds + profiles gate)
├── .env.template                                # modify: add KAFKA_BOOTSTRAP_ADMIN_PASSWORD
├── start.sh                                      # modify: add Kafka health wait + apply + secret sync + first start of gated sidecars
└── kafka/
    ├── kafka_server_jaas.conf                    # new: SCRAM login module declaration, mounted read-only
    └── bootstrap.sh                              # new: format-with-scram-admin-on-first-boot script

infra/
├── modules/
│   ├── kafka-topics/                             # new: for_each over a topics map
│   │   ├── main.tf
│   │   ├── variables.tf
│   │   ├── outputs.tf
│   │   └── versions.tf
│   └── kafka-scram-user/                         # new: one generated credential + ACLs, per service
│       ├── main.tf
│       ├── variables.tf
│       ├── outputs.tf
│       └── versions.tf
└── environments/local/kafka/                     # new: standalone root module, own backend/state
    ├── main.tf
    ├── provider.tf
    ├── variables.tf
    ├── outputs.tf
    ├── backend.tf
    └── versions.tf

Taskfile.yml                                       # modify: infra:*:kafka targets + infra:sync-secrets:kafka

libs/
├── kafka/                                          # new: shared client construction (SASL mechanism + TLS from config)
│   ├── client.go
│   └── client_test.go
├── outbox/producer.go                              # modify: NewProducer takes a *kgo.Client instead of building one, drops AllowAutoTopicCreation
└── inbox/consumer.go                               # modify: NewConsumer takes a fetchClient instead of building one, drops AllowAutoTopicCreation

services/email/cmd/email-received-producer/
├── config.go                                       # modify: add KafkaSaslUsername/KafkaSaslPassword/KafkaSaslMechanism/KafkaTlsEnabled
└── main.go                                         # modify: build client via libs/kafka, pass it to outbox.NewProducer

services/core/cmd/email-received-consumer/
├── config.go                                       # modify: add KafkaSaslUsername/KafkaSaslPassword/KafkaSaslMechanism/KafkaTlsEnabled
└── main.go                                         # modify: build client via libs/kafka, pass it to inbox.NewConsumer
```

---

### Task 1: Kafka broker bootstrap — SASL_PLAINTEXT listener + bootstrap SCRAM admin

**Files:**
- Create: `dev/kafka/kafka_server_jaas.conf`
- Create: `dev/kafka/bootstrap.sh`
- Modify: `dev/docker-compose.yaml` (kafka service only)
- Modify: `dev/.env.template`

**Interfaces:**
- Produces: a Kafka broker reachable at `localhost:9094` (host, for OpenTofu) and `kafka:19094` (Docker network, for sidecars) over `SASL_PLAINTEXT` with `SCRAM-SHA-256`, authenticating as `username=admin` / `password=$KAFKA_BOOTSTRAP_ADMIN_PASSWORD`. Task 2/3 depend on this admin credential to run `tofu apply`. Existing `PLAINTEXT`/`PLAINTEXT_HOST` listeners (ports `19092`/`9092`, used by `kafka-ui`) are unchanged.

This task's sequence was verified empirically (not from documentation alone) against `apache/kafka:4.3.1`, because the image's own docker-entrypoint plumbing formats KRaft storage as a side effect of *generating* `server.properties` from `KAFKA_*` env vars — there is no env var to inject an extra `--add-scram` credential into that step. The only way to get a bootstrap SCRAM credential into a *valid, complete* properties file is: let the stock step generate that file, then wipe the metadata it just wrote and reformat using that same file, this time with `--add-scram`. Done unconditionally, this would wipe topics on every container restart — so it's guarded by checking for `meta.properties` *before* the stock step runs (checking *after* is too late, since the stock step will have already created it).

- [ ] **Step 1: Create the JAAS config**

The stock image's `configure` script has a hard `ensure KAFKA_OPTS` check whenever any `SASL_` listener appears in `KAFKA_ADVERTISED_LISTENERS`, requiring `KAFKA_OPTS` to reference a JAAS file's `java.security.auth.login.config` property. For SCRAM (unlike PLAIN), the file only needs to declare the login module — actual credentials live in the KRaft metadata log, not this file.

`dev/kafka/kafka_server_jaas.conf`:
```
KafkaServer {
   org.apache.kafka.common.security.scram.ScramLoginModule required;
};
```

- [ ] **Step 2: Create the bootstrap script**

`dev/kafka/bootstrap.sh`:
```bash
#!/usr/bin/env bash
set -e

if [ ! -f "$KAFKA_LOG_DIRS/meta.properties" ]; then
  FIRST_BOOT=true
else
  FIRST_BOOT=false
fi

# Generates /opt/kafka/config/server.properties from the KAFKA_* env vars
# (same step the image's default /etc/kafka/docker/run performs) and, as a
# side effect, formats KRaft storage without any SCRAM credential if this
# is the first boot. That side effect is unavoidable: this wrapper doesn't
# expose a way to inject --add-scram into its own format call.
/opt/kafka/bin/kafka-run-class.sh kafka.docker.KafkaDockerWrapper setup \
  --default-configs-dir /etc/kafka/docker \
  --mounted-configs-dir /mnt/shared/config \
  --final-configs-dir /opt/kafka/config

if [ "$FIRST_BOOT" = "true" ]; then
  echo "===> First boot: seeding bootstrap SCRAM admin credential"
  rm -rf "${KAFKA_LOG_DIRS:?}"/*
  /opt/kafka/bin/kafka-storage.sh format \
    --add-scram "SCRAM-SHA-256=[name=admin,password=${KAFKA_BOOTSTRAP_ADMIN_PASSWORD}]" \
    -t "$CLUSTER_ID" \
    -c /opt/kafka/config/server.properties
fi

exec /opt/kafka/bin/kafka-server-start.sh /opt/kafka/config/server.properties
```

- [ ] **Step 3: Update the kafka service in `dev/docker-compose.yaml`**

Replace the current `kafka` service block (the one with `container_name: kafka`, `image: apache/kafka:latest`) with:

```yaml
  kafka:
    container_name: kafka
    image: apache/kafka:4.3.1
    environment:
      KAFKA_NODE_ID: 1
      KAFKA_PROCESS_ROLES: broker,controller
      KAFKA_LOG_DIRS: /var/lib/kafka/data
      KAFKA_LISTENERS: PLAINTEXT://0.0.0.0:19092,CONTROLLER://0.0.0.0:9093,PLAINTEXT_HOST://0.0.0.0:9092,SASL_INTERNAL://0.0.0.0:19094,SASL_HOST://0.0.0.0:9094
      KAFKA_ADVERTISED_LISTENERS: PLAINTEXT://kafka:19092,PLAINTEXT_HOST://localhost:9092,SASL_INTERNAL://kafka:19094,SASL_HOST://localhost:9094
      KAFKA_LISTENER_SECURITY_PROTOCOL_MAP: CONTROLLER:PLAINTEXT,PLAINTEXT:PLAINTEXT,PLAINTEXT_HOST:PLAINTEXT,SASL_INTERNAL:SASL_PLAINTEXT,SASL_HOST:SASL_PLAINTEXT
      KAFKA_CONTROLLER_LISTENER_NAMES: CONTROLLER
      KAFKA_CONTROLLER_QUORUM_VOTERS: 1@localhost:9093
      KAFKA_INTER_BROKER_LISTENER_NAME: PLAINTEXT
      KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR: 1
      KAFKA_AUTO_CREATE_TOPICS_ENABLE: "false"
      KAFKA_SASL_ENABLED_MECHANISMS: SCRAM-SHA-256
      KAFKA_OPTS: -Djava.security.auth.login.config=/etc/kafka/secrets/kafka_server_jaas.conf
      CLUSTER_ID: MkU3OEVBNTcwNTJENDM2Qk
      KAFKA_BOOTSTRAP_ADMIN_PASSWORD: ${KAFKA_BOOTSTRAP_ADMIN_PASSWORD}
    volumes:
      - ./kafka/kafka_server_jaas.conf:/etc/kafka/secrets/kafka_server_jaas.conf:ro
      - ./kafka/bootstrap.sh:/etc/kafka/docker/bootstrap.sh:ro
    entrypoint: ["/bin/bash", "/etc/kafka/docker/bootstrap.sh"]
    ports:
      - 9092:9092
      - 9094:9094
```

Notes on what changed from today's block: `KAFKA_AUTO_CREATE_TOPICS_ENABLE` flips to `"false"` (Task 2 makes topic creation declarative — this is the one line from today's config this plan explicitly removes reliance on, per the spec); a new `9094:9094` host port mapping is added alongside the existing `9092:9092`; `entrypoint` is overridden entirely (the image's default `CMD` is bypassed in favor of `bootstrap.sh`, which still performs everything the default entrypoint does, plus the SCRAM seeding).

- [ ] **Step 4: Add the bootstrap admin password to `dev/.env.template`**

Add to `dev/.env.template` (this is a *consumed* secret, fixed like `OAUTH_CLIENT_SECRET` — not generated by Terraform):
```
KAFKA_BOOTSTRAP_ADMIN_PASSWORD=kafka-admin-dev-password
```

- [ ] **Step 5: Verify — first boot seeds the credential correctly**

```bash
cp dev/.env.template dev/.env   # if dev/.env doesn't already exist
cd dev && docker compose up -d kafka
sleep 15
docker compose logs kafka | grep "First boot"
```
Expected: `===> First boot: seeding bootstrap SCRAM admin credential` appears exactly once.

```bash
docker compose exec kafka sh -c '
cat > /tmp/admin.properties <<EOF
security.protocol=SASL_PLAINTEXT
sasl.mechanism=SCRAM-SHA-256
sasl.jaas.config=org.apache.kafka.common.security.scram.ScramLoginModule required username="admin" password="'"$KAFKA_BOOTSTRAP_ADMIN_PASSWORD"'";
EOF
/opt/kafka/bin/kafka-topics.sh --bootstrap-server localhost:9094 --command-config /tmp/admin.properties --list'
```
Expected: exits cleanly with no topics listed (empty output), not an authentication error.

- [ ] **Step 6: Verify — a restart does not re-seed or wipe data**

```bash
docker compose exec kafka sh -c '/opt/kafka/bin/kafka-topics.sh --bootstrap-server localhost:9094 --command-config /tmp/admin.properties --create --topic verify-persist --partitions 1 --replication-factor 1'
docker compose stop kafka
docker compose start kafka
sleep 15
docker compose logs kafka | grep -c "First boot"
docker compose exec kafka sh -c '/opt/kafka/bin/kafka-topics.sh --bootstrap-server localhost:9094 --command-config /tmp/admin.properties --list'
```
Expected: the `grep -c` count is `1` (not 2 — no re-seed on restart), and `verify-persist` is still listed.

- [ ] **Step 7: Clean up the manual verification topic and commit**

```bash
docker compose exec kafka sh -c '/opt/kafka/bin/kafka-topics.sh --bootstrap-server localhost:9094 --command-config /tmp/admin.properties --delete --topic verify-persist'
git add dev/kafka/kafka_server_jaas.conf dev/kafka/bootstrap.sh dev/docker-compose.yaml dev/.env.template
git commit -m "feat(infra): seed bootstrap SCRAM admin on Kafka broker at KRaft format time"
```

---

### Task 2: `kafka-topics` and `kafka-scram-user` modules + local environment root

**Files:**
- Create: `infra/modules/kafka-topics/{main.tf,variables.tf,outputs.tf,versions.tf}`
- Create: `infra/modules/kafka-scram-user/{main.tf,variables.tf,outputs.tf,versions.tf}`
- Create: `infra/environments/local/kafka/{main.tf,provider.tf,variables.tf,outputs.tf,backend.tf,versions.tf}`

**Interfaces:**
- Consumes: the broker from Task 1, reachable at `localhost:9094` with the bootstrap admin credential.
- Produces: `module.email_received_producer.password` / `module.core_email_received_consumer.password` (sensitive outputs) — Task 3's secret-sync step reads these by name.

- [ ] **Step 1: `kafka-topics` module**

`infra/modules/kafka-topics/versions.tf`:
```hcl
terraform {
  required_providers {
    kafka = {
      source  = "Mongey/kafka"
      version = "~> 0.13"
    }
  }
}
```

`infra/modules/kafka-topics/variables.tf`:
```hcl
variable "topics" {
  description = "topic name => { partitions, replication_factor, config }"
  type = map(object({
    partitions        = number
    replication_factor = number
    config            = optional(map(string), {})
  }))
}
```

`infra/modules/kafka-topics/main.tf`:
```hcl
resource "kafka_topic" "this" {
  for_each = var.topics

  name               = each.key
  partitions         = each.value.partitions
  replication_factor = each.value.replication_factor
  config             = each.value.config
}
```

`infra/modules/kafka-topics/outputs.tf`:
```hcl
output "topic_names" {
  value = [for t in kafka_topic.this : t.name]
}
```

- [ ] **Step 2: `kafka-scram-user` module**

`infra/modules/kafka-scram-user/versions.tf`:
```hcl
terraform {
  required_providers {
    kafka = {
      source  = "Mongey/kafka"
      version = "~> 0.13"
    }
    random = {
      source  = "hashicorp/random"
      version = "~> 3.6"
    }
  }
}
```

`infra/modules/kafka-scram-user/variables.tf`:
```hcl
variable "username" {
  description = "SCRAM username for this Kafka client service"
  type        = string
}

variable "acls" {
  description = "ACL grants for this user, e.g. [{ resource_type = \"Topic\", resource_name = \"email-received\", resource_pattern_type_filter = \"Literal\", operation = \"Write\", permission_type = \"Allow\" }]"
  type = list(object({
    resource_type                = string
    resource_name                = string
    resource_pattern_type_filter = string
    operation                    = string
    permission_type              = string
  }))
}
```

`infra/modules/kafka-scram-user/main.tf`:
```hcl
resource "random_password" "this" {
  length  = 32
  special = false
}

resource "kafka_user_scram_credential" "this" {
  username        = var.username
  scram_mechanism = "SCRAM-SHA-256"
  password        = random_password.this.result
}

resource "kafka_acl" "this" {
  for_each = { for i, a in var.acls : i => a }

  resource_name                = each.value.resource_name
  resource_type                = each.value.resource_type
  resource_pattern_type_filter = each.value.resource_pattern_type_filter
  acl_principal                = "User:${var.username}"
  acl_host                     = "*"
  acl_operation                = each.value.operation
  acl_permission_type          = each.value.permission_type
}
```

`infra/modules/kafka-scram-user/outputs.tf`:
```hcl
output "username" {
  value = kafka_user_scram_credential.this.username
}

output "password" {
  value     = random_password.this.result
  sensitive = true
}
```

- [ ] **Step 3: Local environment root — provider, backend, variables**

`infra/environments/local/kafka/versions.tf`:
```hcl
terraform {
  required_version = ">= 1.6.0"

  required_providers {
    kafka = {
      source  = "Mongey/kafka"
      version = "~> 0.13"
    }
    random = {
      source  = "hashicorp/random"
      version = "~> 3.6"
    }
  }
}
```

`infra/environments/local/kafka/backend.tf` (mirrors `infra/environments/local/keycloak/backend.tf`):
```hcl
terraform {
  backend "local" {
    path = "../../../.tofu-state/local-kafka.tfstate"
  }
}
```

`infra/environments/local/kafka/variables.tf`:
```hcl
variable "kafka_bootstrap_admin_password" {
  description = "Bootstrap SCRAM admin password, must match dev/.env's KAFKA_BOOTSTRAP_ADMIN_PASSWORD"
  type        = string
  sensitive   = true
}
```

`infra/environments/local/kafka/provider.tf`:
```hcl
provider "kafka" {
  bootstrap_servers = ["localhost:9094"]
  tls_enabled       = false
  sasl_mechanism    = "scram-sha256"
  sasl_username     = "admin"
  sasl_password     = var.kafka_bootstrap_admin_password
}
```

- [ ] **Step 4: Local environment root — module calls**

`infra/environments/local/kafka/main.tf`:
```hcl
module "topics" {
  source = "../../../modules/kafka-topics"

  topics = {
    "email-received" = {
      partitions         = 3
      replication_factor = 1
      config = {
        "retention.ms" = "604800000"
      }
    }
  }
}

module "email_received_producer" {
  source = "../../../modules/kafka-scram-user"

  username = "email-received-producer"
  acls = [
    { resource_type = "Topic", resource_name = "email-received", resource_pattern_type_filter = "Literal", operation = "Write", permission_type = "Allow" },
    { resource_type = "Topic", resource_name = "email-received", resource_pattern_type_filter = "Literal", operation = "Describe", permission_type = "Allow" },
  ]

  depends_on = [module.topics]
}

module "core_email_received_consumer" {
  source = "../../../modules/kafka-scram-user"

  username = "core-email-received-consumer"
  acls = [
    { resource_type = "Topic", resource_name = "email-received", resource_pattern_type_filter = "Literal", operation = "Read", permission_type = "Allow" },
    { resource_type = "Topic", resource_name = "email-received", resource_pattern_type_filter = "Literal", operation = "Describe", permission_type = "Allow" },
    { resource_type = "Group", resource_name = "core-email-received-consumer", resource_pattern_type_filter = "Literal", operation = "Read", permission_type = "Allow" },
    { resource_type = "Group", resource_name = "core-email-received-consumer", resource_pattern_type_filter = "Literal", operation = "Describe", permission_type = "Allow" },
  ]

  depends_on = [module.topics]
}
```

`infra/environments/local/kafka/outputs.tf`:
```hcl
output "email_received_producer_password" {
  value     = module.email_received_producer.password
  sensitive = true
}

output "core_email_received_consumer_password" {
  value     = module.core_email_received_consumer.password
  sensitive = true
}
```

- [ ] **Step 5: Add the tfvars value and verify**

Add to `infra/environments/local/kafka/terraform.tfvars` (gitignored, mirrors `infra/environments/local/keycloak/terraform.tfvars`'s existing convention — create this file locally, matching whatever value Task 1 put in `dev/.env`'s `KAFKA_BOOTSTRAP_ADMIN_PASSWORD`):
```hcl
kafka_bootstrap_admin_password = "kafka-admin-dev-password"
```

```bash
cd infra/environments/local/kafka
tofu init
tofu validate
tofu plan
```
Expected: plan shows 1 topic, 2 `kafka_user_scram_credential`, 6 `kafka_acl`, 2 `random_password` to add — 0 to change/destroy.

```bash
tofu apply
tofu output -json | jq '.email_received_producer_password.value, .core_email_received_consumer_password.value'
```
Expected: apply succeeds; both passwords print (non-empty strings).

- [ ] **Step 6: Commit**

```bash
git add infra/modules/kafka-topics infra/modules/kafka-scram-user infra/environments/local/kafka
git commit -m "feat(infra): add kafka-topics and kafka-scram-user OpenTofu modules"
```

---

### Task 3: Taskfile wiring, secret sync, and `task up` sequencing

**Files:**
- Modify: `Taskfile.yml`
- Modify: `dev/start.sh`
- Modify: `dev/docker-compose.yaml` (profile gate on the two sidecar services only — Task 4 edits their `environment:` blocks separately)

**Interfaces:**
- Consumes: `tofu output -json` field names from Task 2 (`email_received_producer_password`, `core_email_received_consumer_password`).
- Produces: `dev/.env` gains `EMAIL_PRODUCER_KAFKA_SASL_PASSWORD` and `CORE_CONSUMER_KAFKA_SASL_PASSWORD` — Task 4's docker-compose changes consume these two exact names.

- [ ] **Step 1: Add Taskfile targets for the Kafka domain**

In `Taskfile.yml`, alongside the existing `infra:*:keycloak` targets, add:
```yaml
  infra:init:kafka:
    desc: Initialize the local Kafka OpenTofu config
    dir: infra/environments/local/kafka
    cmds:
      - tofu init

  infra:validate:kafka:
    desc: Validate the local Kafka OpenTofu config
    dir: infra/environments/local/kafka
    cmds:
      - tofu validate

  infra:plan:kafka:
    desc: Preview changes to the local Kafka config
    dir: infra/environments/local/kafka
    cmds:
      - tofu plan {{.CLI_ARGS}}

  infra:apply:kafka:
    desc: Apply changes to the local Kafka config
    dir: infra/environments/local/kafka
    cmds:
      - tofu apply -input=false -parallelism=1 {{.CLI_ARGS}}
```

(`-parallelism=1` matches the fix already applied to `infra:apply:keycloak` for the same reason: this module's `kafka_acl`/`kafka_user_scram_credential` resources are independent `for_each`/module instances created concurrently by default, which is exactly the shape that caused intermittent 409s on the Keycloak side. Serializing from the start avoids reproducing that bug here.)

- [ ] **Step 2: Add a secret-sync Taskfile target**

```yaml
  infra:sync-secrets:kafka:
    desc: Write generated Kafka SCRAM passwords into dev/.env for docker-compose to pick up
    dir: infra/environments/local/kafka
    cmds:
      - |
        PRODUCER_PW=$(tofu output -raw email_received_producer_password)
        CONSUMER_PW=$(tofu output -raw core_email_received_consumer_password)
        sed -i.bak '/^EMAIL_PRODUCER_KAFKA_SASL_PASSWORD=/d;/^CORE_CONSUMER_KAFKA_SASL_PASSWORD=/d' ../../../../dev/.env
        echo "EMAIL_PRODUCER_KAFKA_SASL_PASSWORD=$PRODUCER_PW" >> ../../../../dev/.env
        echo "CORE_CONSUMER_KAFKA_SASL_PASSWORD=$CONSUMER_PW" >> ../../../../dev/.env
        rm -f ../../../../dev/.env.bak
```

- [ ] **Step 3: Verify the sync target in isolation**

```bash
task infra:sync-secrets:kafka
grep KAFKA_SASL_PASSWORD dev/.env
```
Expected: both lines present with non-empty values, and re-running the task doesn't duplicate them (the `sed` delete-then-append makes it idempotent).

- [ ] **Step 4: Gate the two sidecars behind a Compose profile**

In `dev/docker-compose.yaml`, add a `profiles:` entry to both `email-received-producer` and `email-received-consumer` (each service's top-level keys, alongside its existing `container_name:`/`build:`/etc.):

```yaml
    # Gated behind a profile so a bare `docker compose up` (what start.sh
    # runs first) never starts these with a blank Kafka SASL password.
    # They're started explicitly, by name, once real credentials exist in
    # dev/.env — see start.sh. Naming a service explicitly always starts it
    # regardless of its profile, so this only affects the bare, no-args
    # `docker compose up` case.
    profiles:
      - kafka-clients
```

Add this block to both service definitions. Nothing else in either service's YAML changes in this step (Task 4 Step 5 edits their `environment:` blocks separately).

- [ ] **Step 5: Resequence `dev/start.sh`**

Insert a Kafka health check + apply + secret sync + first start of the two gated sidecars, after the existing Keycloak apply block and before the trailing `wait`:

```bash
# prepare kafka (OpenTofu-managed topics + SCRAM users)
echo "Waiting for Kafka..."
until docker compose exec -T kafka /opt/kafka/bin/kafka-broker-api-versions.sh --bootstrap-server localhost:9092 > /dev/null 2>&1; do
  echo "Waiting for Kafka to be ready..."
  sleep 5
done

echo "Preparing Kafka..."
task infra:apply:kafka -- -auto-approve
task infra:sync-secrets:kafka

# First start of the two Kafka-client sidecars, now that
# task infra:sync-secrets:kafka has written their real SASL passwords into
# dev/.env. They were skipped by the earlier bare `docker compose up`
# because of their `profiles:` entry, so this is their first start, not a
# recreate — they never run with a blank credential.
docker compose up -d --no-deps email-received-producer email-received-consumer
```

The `kafka-broker-api-versions.sh` call is a plain unauthenticated admin request against the existing `PLAINTEXT_HOST` listener (port 9092, unchanged by this plan) — it only proves the broker process is accepting connections, which is all `tofu apply` needs before it can authenticate over the SASL listener.

- [ ] **Step 6: Full-cycle verification**

```bash
task down
task up
```
Expected: completes without error. `docker compose ps` shows `email-received-producer`/`email-received-consumer` as absent right after the first `docker compose up` (before the Kafka health-wait loop) and present only after `task infra:sync-secrets:kafka` runs. `docker compose logs email-received-producer` and `docker compose logs email-received-consumer` show no `SaslAuthenticationException` at all — since these containers are never started before real credentials exist, there is no expected window of auth errors this time (unlike the existing "waiting for the outbox table" pattern, which is a different, already-accepted kind of startup race).

- [ ] **Step 7: Commit**

```bash
git add Taskfile.yml dev/start.sh dev/docker-compose.yaml
git commit -m "feat(infra): wire task up to apply Kafka config and sync generated secrets"
```

---

### Task 4: `libs/kafka` client helper + sidecar SASL wiring

**Files:**
- Create: `libs/kafka/client.go`
- Create: `libs/kafka/client_test.go`
- Modify: `libs/outbox/producer.go`
- Modify: `libs/inbox/consumer.go`
- Modify: `services/email/cmd/email-received-producer/config.go`
- Modify: `services/email/cmd/email-received-producer/main.go`
- Modify: `services/core/cmd/email-received-consumer/config.go`
- Modify: `services/core/cmd/email-received-consumer/main.go`
- Modify: `dev/docker-compose.yaml` (the two sidecar services only)

**Interfaces:**
- Consumes: `EMAIL_PRODUCER_KAFKA_SASL_PASSWORD` / `CORE_CONSUMER_KAFKA_SASL_PASSWORD` from `dev/.env` (Task 3), usernames `email-received-producer` / `core-email-received-consumer` (Task 2, fixed strings, not secret).
- Produces: `kafka.Config` + `kafka.NewClient(cfg Config, extraOpts ...kgo.Opt) (*kgo.Client, error)` — the one place SASL mechanism and TLS are chosen, so switching a real environment to `SASL_SSL`/`SCRAM-SHA-512` is a `KAFKA_TLS_ENABLED`/`KAFKA_SASL_MECHANISM` env-var change, not a code change. `outbox.NewProducer(client syncProducer) Producer` and `inbox.NewConsumer(client fetchClient, topic string, store saver) *Consumer` both now take an already-constructed client instead of building their own — a real `*kgo.Client` satisfies both narrow interfaces structurally, so both `main.go`s just pass the `*kgo.Client` `kafka.NewClient` returns, no explicit conversion needed. Construction lives in one place (`libs/kafka`), driven by each sidecar's own `main.go`.

This task answers a question raised while reviewing this plan: the original design hardcoded `.AsSha256Mechanism()` directly in both `libs/outbox` and `libs/inbox`, with no TLS option at all, meaning a future switch to `SASL_SSL`/`SCRAM-SHA-512` would require editing and redeploying those files. `libs/kafka` centralizes that choice behind config instead. `libs/outbox`'s `Producer` interface and `libs/inbox`'s `saver` interface already gave each package a narrow, franz-go-independent contract for its own downstream adapters — this task extends the same idea to *construction*: neither package needs to know how to build an authenticated `kgo.Client` anymore, only how to use one it's given.

- [ ] **Step 1: `libs/kafka/client.go`**

```go
package kafka

import (
	"crypto/tls"
	"fmt"

	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/sasl"
	"github.com/twmb/franz-go/pkg/sasl/scram"
)

// Config is the shared connection configuration for any Kafka client this
// app creates. Which SASL/SCRAM mechanism to use and whether to dial with
// TLS are config, not a compile-time choice, so moving from this repo's
// local SASL_PLAINTEXT/SCRAM-SHA-256 setup to a production SASL_SSL/
// SCRAM-SHA-512 broker never requires a code change in a producer or
// consumer - only these fields' values change.
type Config struct {
	Brokers    []string
	Username   string
	Password   string
	Mechanism  string // "SCRAM-SHA-256" or "SCRAM-SHA-512"
	TLSEnabled bool
}

// NewClient builds a franz-go client authenticated per cfg. extraOpts lets
// a caller layer its own options (e.g. inbox's consumer-group settings) on
// top of the shared broker/SASL/TLS configuration - this package only ever
// builds the connection, never decides what a caller does with it.
func NewClient(cfg Config, extraOpts ...kgo.Opt) (*kgo.Client, error) {
	mechanism, err := saslMechanism(cfg.Mechanism, cfg.Username, cfg.Password)
	if err != nil {
		return nil, err
	}

	opts := []kgo.Opt{
		kgo.SeedBrokers(cfg.Brokers...),
		kgo.SASL(mechanism),
	}
	if cfg.TLSEnabled {
		opts = append(opts, kgo.DialTLSConfig(&tls.Config{}))
	}
	opts = append(opts, extraOpts...)

	return kgo.NewClient(opts...)
}

func saslMechanism(mechanism, username, password string) (sasl.Mechanism, error) {
	auth := scram.Auth{User: username, Pass: password}
	switch mechanism {
	case "SCRAM-SHA-256":
		return auth.AsSha256Mechanism(), nil
	case "SCRAM-SHA-512":
		return auth.AsSha512Mechanism(), nil
	default:
		return nil, fmt.Errorf("unsupported KAFKA_SASL_MECHANISM %q: must be SCRAM-SHA-256 or SCRAM-SHA-512", mechanism)
	}
}
```

- [ ] **Step 2: `libs/kafka/client_test.go`**

`NewClient` itself needs a live broker to test meaningfully (it doesn't fail on bad credentials - `kgo.NewClient` connects lazily, confirmed in Global Constraints), but `saslMechanism` is a pure function and the one piece of new branching logic this task adds - test that directly, without a broker:

```go
package kafka

import "testing"

func TestSaslMechanism(t *testing.T) {
	t.Run("SCRAM-SHA-256 succeeds", func(t *testing.T) {
		if _, err := saslMechanism("SCRAM-SHA-256", "user", "pass"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("SCRAM-SHA-512 succeeds", func(t *testing.T) {
		if _, err := saslMechanism("SCRAM-SHA-512", "user", "pass"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("unsupported mechanism errors", func(t *testing.T) {
		if _, err := saslMechanism("PLAIN", "user", "pass"); err == nil {
			t.Fatal("expected an error for an unsupported mechanism, got nil")
		}
	})
}
```

Run: `go test ./libs/kafka/... -v` — expect all three subtests to pass.

- [ ] **Step 3: Simplify `libs/outbox/producer.go` to take a client instead of building one**

```go
package outbox

import (
	"context"

	"github.com/twmb/franz-go/pkg/kgo"
)

// Producer sends a single record to Kafka. Implemented by kafkaProducer in
// production; test code can supply a fake so an adapter's poll/mark logic is
// unit-testable without a live broker.
type Producer interface {
	ProduceSync(ctx context.Context, topic string, key, value []byte) error
}

// syncProducer is the narrow slice of *kgo.Client's API kafkaProducer needs -
// declared here (not imported) so kafkaProducer can be driven by a fake in
// tests without a live broker. *kgo.Client (built via planeo/libs/kafka.NewClient)
// satisfies this structurally; production code never constructs a
// syncProducer value directly, it passes a real *kgo.Client.
type syncProducer interface {
	ProduceSync(ctx context.Context, rs ...*kgo.Record) kgo.ProduceResults
}

type kafkaProducer struct {
	client syncProducer
}

func (p *kafkaProducer) ProduceSync(ctx context.Context, topic string, key, value []byte) error {
	result := p.client.ProduceSync(ctx, &kgo.Record{Topic: topic, Key: key, Value: value})
	return result.FirstErr()
}

// NewProducer wraps an already-authenticated *kgo.Client (see
// planeo/libs/kafka.NewClient) as a Producer. This package no longer
// constructs or owns the client's lifecycle - the caller does, and is
// responsible for calling client.Close() on shutdown.
func NewProducer(client syncProducer) Producer {
	return &kafkaProducer{client: client}
}
```

`kgo.AllowAutoTopicCreation()` is gone (not carried over from today's version): the topic is now Terraform-managed and broker-side auto-create is off per Task 1's `KAFKA_AUTO_CREATE_TOPICS_ENABLE: "false"` — this option would be a silent no-op against a broker that refuses to honor it. `NewProducer` no longer returns an `error` at all: wrapping an already-built client can't fail. `syncProducer` mirrors Step 4's `fetchClient` on the inbox side — both packages now depend on a narrow, locally-declared interface rather than the concrete `*kgo.Client`, kept symmetric on purpose.

- [ ] **Step 4: Narrow `libs/inbox/consumer.go` to a `fetchClient` interface, and take a client instead of building one**

```go
package inbox

import (
	"context"

	"planeo/libs/logger"

	"github.com/twmb/franz-go/pkg/kgo"
)

type saver interface {
	Save(ctx context.Context, topic string, partition int32, offset int64, payload []byte) (inserted bool, err error)
}

// fetchClient is the narrow slice of *kgo.Client's API Consumer needs -
// declared here (not imported) so Run can be driven by a fake in tests
// without a live broker. *kgo.Client (built via planeo/libs/kafka.NewClient)
// satisfies this structurally; production code never constructs a
// fetchClient value directly, it passes a real *kgo.Client.
type fetchClient interface {
	PollFetches(ctx context.Context) kgo.Fetches
	CommitRecords(ctx context.Context, rs ...*kgo.Record) error
	Close()
}

// Consumer reads from Kafka and persists into the inbox, committing the
// offset only after Save succeeds. No Handler is invoked here. The given
// client must already be configured with its consumer group and topic
// subscription (via kgo.ConsumerGroup/kgo.ConsumeTopics passed as extraOpts
// to planeo/libs/kafka.NewClient) - Consumer only drives it, it doesn't
// configure it.
type Consumer struct {
	client fetchClient
	topic  string
	store  saver
}

func NewConsumer(client fetchClient, topic string, store saver) *Consumer {
	return &Consumer{
		client: client,
		topic:  topic,
		store:  store,
	}
}

// Run subscribes to the configured topic and persists each fetched record
// into the inbox, deduped on the Kafka coordinate, committing the offset
// only after a successful Save. Non-blocking — starts its own background
// goroutine and returns immediately.
func (c *Consumer) Run(ctx context.Context) error {
	log := logger.FromContext(ctx)

	go func() {
		defer c.client.Close()

		for {
			if ctx.Err() != nil {
				return
			}

			fetches := c.client.PollFetches(ctx)
			if fetches.IsClientClosed() {
				return
			}

			fetches.EachError(func(_ string, _ int32, err error) {
				log.Error().Err(err).Msg("kafka fetch error")
			})

			// Kafka offset commits are a single cumulative per-partition
			// marker, not a per-record acknowledgement: if record N's Save
			// fails but record N+1's Save (and commit) succeeds, the
			// committed offset would advance past N, and N would never be
			// redelivered — a silent, permanent loss of exactly the message
			// this pattern exists to protect. To prevent that, once a
			// partition has a failure in this poll, stop committing further
			// records on that partition for the rest of the poll (Save's
			// (topic, partition, offset) dedup makes it safe to reprocess
			// already-saved records on the next poll/redelivery).
			failedPartitions := map[int32]bool{}
			fetches.EachRecord(func(record *kgo.Record) {
				if failedPartitions[record.Partition] {
					return
				}

				if _, err := c.store.Save(ctx, c.topic, record.Partition, record.Offset, record.Value); err != nil {
					log.Error().Err(err).Int32("partition", record.Partition).Int64("offset", record.Offset).Msg("failed to persist inbox record, halting commits on this partition until next poll")
					failedPartitions[record.Partition] = true
					return
				}

				if err := c.client.CommitRecords(ctx, record); err != nil {
					log.Error().Err(err).Int32("partition", record.Partition).Int64("offset", record.Offset).Msg("failed to commit kafka offset, halting further commits on this partition")
					failedPartitions[record.Partition] = true
				}
			})
		}
	}()

	return nil
}
```

`brokers`, `groupName`, and `username`/`password` are gone from `Consumer` entirely — consumer-group membership and topic subscription are now `kgo.ConsumerGroup`/`kgo.ConsumeTopics` options the caller passes to `kafka.NewClient` (Step 6), and authentication is `kafka.NewClient`'s concern, not `Consumer`'s. `topic` stays, since `Run` still needs it for the `store.Save` call. The polling/commit/partition-failure-tracking logic itself is unchanged from today's version.

- [ ] **Step 5: Wire the email producer sidecar**

In `services/email/cmd/email-received-producer/config.go`, add four fields to `Config` (two SASL identity fields plus the two new config-driven ones) and load them in `LoadConfig`. Also add a `readBoolEnvVariableWithDefault` helper (this file has no bool-reading helper today):
```go
type Config struct {
	DbHost             string
	DbPort             string
	DbUser             string
	DbPassword         string
	DbName             string
	KafkaBrokers       string
	KafkaSaslUsername  string
	KafkaSaslPassword  string
	KafkaSaslMechanism string
	KafkaTlsEnabled    bool
	PollInterval       time.Duration
	BatchSize          int
	MaxAttempts        int
	ClaimTTL           time.Duration
}
```
```go
	return &Config{
		DbHost:             readEnvVariable(ctx, "DB_HOST"),
		DbPort:              readEnvVariable(ctx, "DB_PORT"),
		DbUser:              readEnvVariable(ctx, "DB_USER"),
		DbPassword:          readEnvVariable(ctx, "DB_PASSWORD"),
		DbName:              readEnvVariable(ctx, "DB_NAME"),
		KafkaBrokers:        readEnvVariable(ctx, "KAFKA_BROKERS"),
		KafkaSaslUsername:   readEnvVariable(ctx, "KAFKA_SASL_USERNAME"),
		KafkaSaslPassword:   readEnvVariable(ctx, "KAFKA_SASL_PASSWORD"),
		KafkaSaslMechanism:  readStringEnvVariableWithDefault("KAFKA_SASL_MECHANISM", "SCRAM-SHA-256"),
		KafkaTlsEnabled:     readBoolEnvVariableWithDefault("KAFKA_TLS_ENABLED", false),
		PollInterval:        readDurationEnvVariable(ctx, "EMAIL_RECEIVED_PRODUCER_POLL_INTERVAL", 1*time.Second),
		BatchSize:           readIntEnvVariable(ctx, "EMAIL_RECEIVED_PRODUCER_BATCH_SIZE", 100),
		MaxAttempts:         readIntEnvVariable(ctx, "EMAIL_RECEIVED_PRODUCER_MAX_ATTEMPTS", 5),
		ClaimTTL:            readDurationEnvVariable(ctx, "EMAIL_RECEIVED_PRODUCER_CLAIM_TTL", 30*time.Second),
	}
```
```go
func readStringEnvVariableWithDefault(name string, def string) string {
	v, ok := os.LookupEnv(name)
	if !ok {
		return def
	}
	return v
}

func readBoolEnvVariableWithDefault(name string, def bool) bool {
	v, ok := os.LookupEnv(name)
	if !ok {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}
```

In `services/email/cmd/email-received-producer/main.go`, add `"planeo/libs/kafka"` to the imports and change the Kafka setup:
```go
	brokers := strings.Split(cfg.KafkaBrokers, ",")
	kafkaClient, err := kafka.NewClient(kafka.Config{
		Brokers:    brokers,
		Username:   cfg.KafkaSaslUsername,
		Password:   cfg.KafkaSaslPassword,
		Mechanism:  cfg.KafkaSaslMechanism,
		TLSEnabled: cfg.KafkaTlsEnabled,
	})
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to Kafka")
	}
	defer kafkaClient.Close()

	producer := outbox.NewProducer(kafkaClient)
```

- [ ] **Step 6: Wire the core consumer sidecar**

In `services/core/cmd/email-received-consumer/config.go`, add the same four fields to `Config`, loaded the same way (this file already has a `readStringEnvVariableWithDefault` helper for `GroupName` — reuse it for `KafkaSaslMechanism`; add `readBoolEnvVariableWithDefault` the same way as Step 5).

In `services/core/cmd/email-received-consumer/main.go`, add `"planeo/libs/kafka"` and `"github.com/twmb/franz-go/pkg/kgo"` to the imports and change the Kafka setup:
```go
	brokers := strings.Split(cfg.KafkaBrokers, ",")
	kafkaClient, err := kafka.NewClient(
		kafka.Config{
			Brokers:    brokers,
			Username:   cfg.KafkaSaslUsername,
			Password:   cfg.KafkaSaslPassword,
			Mechanism:  cfg.KafkaSaslMechanism,
			TLSEnabled: cfg.KafkaTlsEnabled,
		},
		kgo.ConsumerGroup(cfg.GroupName),
		kgo.ConsumeTopics(contracts.EmailReceivedTopic),
		kgo.DisableAutoCommit(),
	)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to Kafka")
	}

	consumer := inbox.NewConsumer(kafkaClient, contracts.EmailReceivedTopic, db)
	if err := consumer.Run(runCtx); err != nil {
		log.Fatal().Err(err).Msg("Failed to start email-received consumer")
	}
```
(`runCtx` is already defined earlier in this file, via the existing `signal.NotifyContext` call — unchanged.)

- [ ] **Step 7: Update the two sidecar service blocks in `dev/docker-compose.yaml`**

In `email-received-producer`'s `environment:` block, change `KAFKA_BROKERS: kafka:19092` to `KAFKA_BROKERS: kafka:19094` (the new internal SASL listener from Task 1) and add:
```yaml
      KAFKA_SASL_USERNAME: email-received-producer
      KAFKA_SASL_PASSWORD: ${EMAIL_PRODUCER_KAFKA_SASL_PASSWORD}
      KAFKA_SASL_MECHANISM: SCRAM-SHA-256
      KAFKA_TLS_ENABLED: "false"
```

In `email-received-consumer`'s `environment:` block, the same `KAFKA_BROKERS` change, plus:
```yaml
      KAFKA_SASL_USERNAME: core-email-received-consumer
      KAFKA_SASL_PASSWORD: ${CORE_CONSUMER_KAFKA_SASL_PASSWORD}
      KAFKA_SASL_MECHANISM: SCRAM-SHA-256
      KAFKA_TLS_ENABLED: "false"
```

`KAFKA_SASL_MECHANISM`/`KAFKA_TLS_ENABLED` are set explicitly here (not left to their code defaults) so that `dev`/`stage`/`prod` environments, when they're built, override exactly these two lines to switch to `SCRAM-SHA-512`/`SASL_SSL` — no Go code changes, per this task's whole point.

- [ ] **Step 8: Update existing unit tests for the new signatures**

```bash
grep -rn "NewProducer\|NewConsumer" --include="*_test.go" libs/ services/
```
Expected: no matches outside the two `main.go` call sites already updated in Steps 5–6 (both `Producer` and `saver`/`fetchClient` consumers are tested via fakes against those interfaces, not against these constructors — confirmed unchanged by this refactor). If any test *does* call these constructors directly, update its call site to match the new signatures.

- [ ] **Step 9: Build and run existing tests**

```bash
go build ./...
go test ./libs/kafka/... -v
task test:core:unit
task test:email:unit
```
Expected: clean build; the three new `TestSaslMechanism` subtests pass; all existing unit tests pass unchanged (this task touches no domain logic, only Kafka client construction and its call sites).

- [ ] **Step 10: Commit**

```bash
git add libs/kafka libs/outbox/producer.go libs/inbox/consumer.go \
  services/email/cmd/email-received-producer/config.go services/email/cmd/email-received-producer/main.go \
  services/core/cmd/email-received-consumer/config.go services/core/cmd/email-received-consumer/main.go \
  dev/docker-compose.yaml
git commit -m "feat: add libs/kafka client helper, wire email-received sidecars via SASL/SCRAM"
```

---

### Task 5: Full workspace verification

**Files:** none (verification only)

**Interfaces:** none — this task proves Tasks 1–4 work together end-to-end.

- [ ] **Step 1: Clean full-cycle bring-up**

```bash
task down
task up
```
Expected: no errors; completes through `migrate:core`/`migrate:email` as it does today.

- [ ] **Step 2: Confirm topics are Terraform-managed, not auto-created**

```bash
cd infra/environments/local/kafka && tofu plan
```
Expected: `No changes.` (proves the topic that exists in the running broker is the one Terraform created, not one auto-created by a race before `tofu apply` ran).

- [ ] **Step 3: Confirm the real email flow still works end-to-end**

Send a test email through the existing local mail setup (per this repo's dev credentials / `greenmail`/`roundcube` services already in `dev/docker-compose.yaml`) and confirm it produces a `Request` in the core service, exactly as it does today — this proves the producer and consumer sidecars are authenticating and moving real messages, not just that the broker accepts connections.

```bash
docker compose -f dev/docker-compose.yaml logs email-received-producer --tail 50
docker compose -f dev/docker-compose.yaml logs email-received-consumer --tail 50
```
Expected: no `SaslAuthenticationException` in either log at all — both containers are profile-gated (Task 3) so they never start before real credentials exist.

- [ ] **Step 4: Confirm `kafka-ui` still works unauthenticated**

Open `http://localhost:8082` and confirm topics/messages are visible. `kafka-ui`'s `KAFKA_CLUSTERS_0_BOOTSTRAPSERVERS: kafka:19092` (the unauthenticated internal `PLAINTEXT` listener) is intentionally unchanged by this plan — it's a local dev inspection tool, not one of the two Kafka client services this plan scopes credentials to.

- [ ] **Step 5: Update `docs/superpowers/specs/2026-07-16-iac-opentofu-foundation-design.md` status**

Add a short note to the end of Section 6 recording that the Kafka half is now implemented (mirroring how Section 2's correction note was added for the Keycloak half):
```markdown

**Implementation note (2026-07-22):** implemented per `docs/superpowers/plans/2026-07-22-kafka-iac-local.md`. One deviation from this section's original sketch: the bootstrap SASL admin credential could not be injected via the stock `apache/kafka` image's env-var-driven config generation (no such hook exists) — it required reformatting KRaft storage a second time, guarded to run only on first boot. See that plan's Task 1 for the verified sequence.
```

- [ ] **Step 6: Commit**

```bash
git add docs/superpowers/specs/2026-07-16-iac-opentofu-foundation-design.md
git commit -m "docs: record Kafka IaC implementation against the OpenTofu foundation spec"
```

## Self-Review

**Spec coverage:** Section 1 (repo structure) → Task 2's file structure. Section 6 (Kafka module design, broker change, kafka-topics, kafka-scram-user) → Tasks 1–2. Section 7 (rollout: local only) → Global Constraints. Section 8 (`task up` integration, wait-for-Kafka-health) → Task 3. Section 9 (Taskfile command reference conventions) → Task 3 Step 1. No spec section left unaddressed.

**Placeholder scan:** no TBD/TODO; every step has literal code, exact commands, and expected output strings.

**Type/name consistency:** `email-received-producer`/`core-email-received-consumer` usernames match across Task 2 (Terraform `username` args), Task 3 secret var names, and Task 4 (docker-compose `KAFKA_SASL_USERNAME` literals) — checked. `EMAIL_PRODUCER_KAFKA_SASL_PASSWORD`/`CORE_CONSUMER_KAFKA_SASL_PASSWORD` match between Task 3's sync script and Task 4's docker-compose `${...}` references — checked. `kafka.Config`/`kafka.NewClient` signature matches between Task 4 Step 1 (library) and Steps 5–6 (both `main.go` call sites) — checked. `outbox.NewProducer(client syncProducer) Producer` and `inbox.NewConsumer(client fetchClient, topic string, store saver) *Consumer` match between Task 4 Steps 3–4 (library) and Steps 5–6 (call sites) — checked; both interfaces are satisfied structurally by the concrete `*kgo.Client` `kafka.NewClient` returns, so neither call site needs an explicit type. `KafkaSaslMechanism`/`KafkaTlsEnabled` field names and `KAFKA_SASL_MECHANISM`/`KAFKA_TLS_ENABLED` env var names match between both config.go edits and both docker-compose service blocks in Task 4 Step 7 — checked. Topic name `email-received` used consistently (not the spec's illustrative `email.received`) — checked.
