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

Create the first admin (uses the `ADMIN_*` values in `server/.env`, safe to run again):

```sh
mise run seed
```

Run the server and the web app:

```sh
mise run dev
```

Log in at http://localhost:5173 with the admin username and password from `server/.env`. Create judges on the Users page, then rubrics, interviews and candidates, and assign judges to interviews.

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

### Sandbox

Use the sandbox for test interviews and candidates. It is a separate database (`tabulation_sandbox`), so it never mixes with the real data in `tabulation`.

Add `-E sandbox` to any task:

```sh
mise -E sandbox run seed:sample   # admin, rubric, 10 candidates, 1-on-1 (judge1-3) and panel (judge4-6) interviews; judge password "password"
mise -E sandbox run dev    # or: mise -E sandbox run start
```

Leave out `-E sandbox` to use the real data. The override lives in `mise.sandbox.toml`.

### Testing

```sh
mise run test
```

Runs lint, the Go tests, Vitest and the Playwright end-to-end test. Requires Docker to be running.

- Go integration tests start their own Postgres container with [Testcontainers](https://testcontainers.com/).
- The e2e test starts its own API (port 8081) and web app (port 5174) against a throwaway database named `tabulation_e2e` in the Postgres from `mise run db:start`. It never touches your dev data. `mise run deps` installs the Chromium it uses.

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
