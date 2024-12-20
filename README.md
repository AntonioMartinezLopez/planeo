# planeo

**WIP**

AI-driven process management platform tailored for various service providers, such as electricity and logistics companies. The platform utilizes AI to oversee order acceptance, planning, and task assignments to employees, optimizing efficiency. Additionally, it features a custom Domain-Specific Language (DSL) that allows users to easily create and configure new workflows that the AI will follow.

## Development

### Prerequisites ###
- Golang > 1.23.0 installed
- Docker and Docker compose installed
- `.env` file generated from `.env.template`

### Preparing the development environment

- `dev/install.sh`: This script checks and installs all dependencies needed for running the development environment
- `dev/start.sh`: This scripts starts the development environment.
- run migrations: see [Database migrations](#database-migrations)

### Generating Access Tokens (For backend testing)

- `backend/get_test_token.sh`: This script is used to login to the local instance realm using client credentials grant. You can either login as Admin, Planner or User

<br>
<center>

| Username              | Role      | Password  |
|-----------------------|---------- |---------- |
| `admin@local.de`      | Admin     | `admin`   |
| `planner@local.de`    | Planner   | `planner` |
| `user@local.de`       | User      | `user`    |

</center>

### Database migrations

Migrations files can be found under `db/migrations`. For actually conducting migrations and initialize all tables and fixtures, goose is used (https://github.com/pressly/goose). By default, the project provides a `.envrc.template` file with environmental variables that goose uses to connect to the database.

#### commands

- `goose status`: shows the recent status (pending or conducted) of all migrations files within the folder
- `goose up`: runs all migrations to the databse specified in the env variables
- `goose down`: reverts all migrations by running the `down` function of each migration file

for more commands see: https://github.com/pressly/goose?tab=readme-ov-file#usage

#### Run  migrations in the dev environment

1. start all containers and processes using `dev/start.sh`
2. `source` the `.envrc` file in order to create environmental variables (or use something like `direnv` to automatically load those when enetering the `db/migrations` directory)
2. Run `goose up`

## Documentation

- Swagger docs can be opened during running dev environment under following link: http://localhost:8888/api/docs#/