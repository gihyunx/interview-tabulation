# Agent Guide

Rules for agents working in this repo. See `README.md` for setup.

## Commands

All commands are mise tasks (`mise tasks` lists them).

```sh
mise run setup            # deps + database + migrations
mise run dev              # database, server, web
mise run lint
mise run format
mise run test             # needs Docker
mise run gen:api          # regenerate web API types
mise run db:create <name> # new migration
```

## Rules

- Migrations: create files only with `mise run db:create <name>`. Never hand-write, rename, or edit an applied migration; add a new one.
- Go: no exported names unless another package needs them.
- SQL: pgx named args (`@name`) everywhere. Never build SQL with string concatenation.
- Tailwind: built-in classes and shadcn theme classes. No arbitrary values unless unavoidable, and then leave a comment saying why.
- Docs: plain keyboard characters only (no em dash, arrows, section sign).
