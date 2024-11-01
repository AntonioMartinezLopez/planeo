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

## Documentation

- Swagger docs can be opened during running dev environment under following link: http://localhost:8888/api/docs#/