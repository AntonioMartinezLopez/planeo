# planeo

**WIP**

AI-driven process management platform tailored for various service providers, such as electricity and logistics companies. The platform utilizes AI to oversee order acceptance, planning, and task assignments to employees, optimizing efficiency. Additionally, it features a custom Domain-Specific Language (DSL) that allows users to easily create and configure new workflows that the AI will follow.

## Development

### Prerequisites

- Golang >= 1.24.5 installed
- Docker and Docker compose installed
- Task (https://taskfile.dev) - Install via `brew install go-task`
- `.env` file generated from `.env.template`

### Preparing the development environment

- `task setup`: Install all dependencies needed for running the development environment
- `task up`: Start the development environment and run migrations
- `task run:core` or `task run:email`: Start a specific service with hot-reload
- `task --list`: Show all available tasks
- Run additional migrations: see [Database migrations](#database-migrations)

> **Note**: Legacy `make` commands are still available but deprecated. New development should use `task`.

### Generating Access Tokens (For backend testing)

- `task login`: Login to the local instance realm using client credentials grant. You can either login as Admin, Planner or User

<br>
<center>

| Username           | Role    | Password  |
| ------------------ | ------- | --------- |
| `admin@local.de`   | Admin   | `admin`   |
| `planner@local.de` | Planner | `planner` |
| `user@local.de`    | User    | `user`    |

</center>

### Database migrations

Migrations files can be found under `<service>/db/migrations`. For actually conducting migrations and initialize all tables and fixtures, goose is used (https://github.com/pressly/goose). By default, the project provides a `.envrc.template` file with environmental variables that goose uses to connect to the database.

#### commands

- `goose status`: shows the recent status (pending or conducted) of all migrations files within the folder
- `goose up`: runs all migrations to the databse specified in the env variables
- `goose down`: reverts all migrations by running the `down` function of each migration file

for more commands see: https://github.com/pressly/goose?tab=readme-ov-file#usage

#### Run migrations in the dev environment

1. Start all containers using `task up` (migrations run automatically)
2. Start services using `task run:core` or `task run:email`
3. For manual migration control:
   - `task migrate:core:status` - Check core migration status
   - `task migrate:core` or `task migrate:email` - Run migrations manually
   - `task migrate:core:down` - Rollback last migration

Alternatively, use goose directly:
1. `source` the `.envrc` file in order to create environmental variables
2. Run `goose up`

## Testing

### Run service unit tests

```bash
# for running all unit tests
go test ./... -v -short

# or for a particular test suite
go test ./... -run TestUser

# or for a particular sub test
go test ./... -run TestUser/GetUser
```

#### Generate mocks for larger interfaces

This project uses mockery (https://github.com/vektra/mockery) to auto-generate mocks. The configuration can be found in `backend/.mockery.yaml`.

In order to create new mocks or update existing ones, specify corresponding in the configuration file and run the mockery binary

```bash
cd backend && mockery
```

### Run backend integration tests

This project uses testcontainers to spin up all depending services. When writing new tests, use the `NewIntegrationTestEnvironment` method to spin up a fresh test environment, which provides everything needed.

```bash
# for running all integration tests
go test ./...

# or for a particular test suite
go test ./... -run TestUserIntegration

# or for a particular sub test
go test ./... -run TestUserIntegration/DELETE_/admin/users
```

## Build Tool

This project uses [Task](https://taskfile.dev) for build automation. Run `task --list` to see all available commands.

For details on the migration from Make to Task, see [docs/MAKEFILE_MIGRATION.md](docs/MAKEFILE_MIGRATION.md).

## Documentation

- Swagger docs can be opened during running dev environment under following link: http://localhost:8000/api/docs#/
