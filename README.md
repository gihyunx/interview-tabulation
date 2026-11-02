# Interview Tabulation System

Tabulation system for a mock interview.

## Prerequisites

- [Docker](https://www.docker.com/)
- [mise](https://mise.jdx.dev/)

Everything else (Go, Node, pnpm, golangci-lint, goose, lefthook) is installed by mise at the versions pinned in `mise.toml`.

## Getting Started

Create the env files for the server and the web app:

```sh
cp server/.env.example server/.env
cp web/.env.example web/.env
```

Install the tools:

```sh
mise install
```

> [!NOTE]
> We use mise tasks for the commands below. See [Tasks](#tasks) for more information.

Install dependencies, start the database, and run migrations:

```sh
mise run setup
```

Run the server and the web app:

```sh
mise run dev
```

| URL                          | What               |
| ---------------------------- | ------------------ |
| http://localhost:5173        | Web app (dev)      |
| http://localhost:8080/health | API health check   |
| http://localhost:8080/docs   | API docs (OpenAPI) |

### Database

Postgres runs in Docker. Data is kept in a named volume, so it survives `mise run db:stop` and restarts.

To create a new migration:

```sh
mise run db:create <name>
```

> [!IMPORTANT]
> Always create migration files with this command. Never write or rename them by hand, and never edit a migration that was already applied. Add a new one instead.

### Testing

```sh
mise run test
```

Requires Docker to be running. Integration tests start their own Postgres container with [Testcontainers](https://testcontainers.com/).

### Tasks

To view the available tasks:

```sh
mise tasks
```

To print more information about a task:

```sh
mise run <task_name> -h
# e.g., mise run db:create -h
```

To run a task:

```sh
mise run <task_name>
# e.g., mise run lint
```

> [!NOTE]
> It is still possible to run the commands without mise. However, to avoid issues caused by version mismatches, using mise is recommended.
