# umrahservice-api (Go)

A standalone Go reimplementation of the Umrah service **mobile API** (the
`routes/api.php` surface of the Laravel app at `../../Sites/umrahservice`).

It is a drop-in replacement: it runs against the **same MariaDB database** and
**interoperates with existing Laravel Sanctum tokens** (`personal_access_tokens`,
SHA-256 scheme), so the mobile app can point at this service unchanged.

- **Router:** Gin
- **ORM:** GORM (MySQL/MariaDB driver) — schema is owned by Laravel; no migrations here
- **Auth:** Sanctum bearer tokens + Spatie roles/permissions (read from existing tables)
- **Storage:** S3 (path-style endpoint), matching `Storage::disk('s3')`
- **PDF:** Gotenberg (group PIF)
- **Broadcasting:** Pusher/Reverb auth signatures + `MessageSent` event

## Layout

```
cmd/server          entrypoint
internal/config     env loading (.env via godotenv)
internal/db         GORM connection
internal/models     GORM structs (mirror Laravel tables exactly)
internal/enums      string-backed enums + Label() maps
internal/auth       Sanctum token middleware, Spatie abilities, policies
internal/storage    S3 client (PutObject + URL)
internal/pdf        Gotenberg client + PIF template
internal/broadcast  Pusher/Reverb signing + event trigger
internal/handlers   one file per Laravel controller
internal/router     route table (mounted under /api)
```

## Run

```bash
cp .env.example .env   # or edit .env (already present for local)
go run ./cmd/server    # listens on $PORT (default 8000), /api/*
```

Requires the MariaDB database `umrahservice_app` to be reachable with the
configured credentials. For the group PIF endpoint, a Gotenberg server is needed:

```bash
docker run --rm -p 3000:3000 gotenberg/gotenberg:8
```

## Endpoints

All of `routes/api.php`: `/api/login`, `/api/logout`, `/api/broadcasting/auth`,
`/api/user`, `/api/user/profile`, `/api/wallet/*`, `/api/groups`,
`/api/groups/:id` (+ files), `/api/luggage-tag/*code`, `/api/schedules`,
`/api/tasks` (+ complete/checklist/messages), `/api/incidents` (+ messages),
`/api/messages/:id`, `/api/notifications/*`.

## Test

```bash
go test ./...   # unit tests for signing, token hashing, abilities, helpers
```

### Integration tests (against the dev DB)

Tests under `test/integration/` boot the full Gin engine and hit endpoints
against a live database. They are gated behind the `integration` build tag, so
the plain `go test ./...` run above stays fast and DB-free.

```bash
go test -tags=integration ./test/integration/...
```

Connection credentials come from the project-root `.env`, but the schema is
forced to the dedicated test database `umrahservice_app_test` (never the app
DB). Override per-run with `DB_DATABASE`:

```bash
DB_DATABASE=some_other_schema go test -tags=integration ./test/integration/...
```

The tests authenticate as the first existing user, seed a real Sanctum token,
and delete it again on cleanup — no data is left behind. If the DB is
unreachable they `t.Skip` rather than fail.

### End-to-end (against the live DB)

Authenticated endpoints need a valid bearer token. Either:

1. `POST /api/login` with a real user's email/password (issues a Sanctum token), or
2. reuse an existing plaintext token from the mobile app.

Then: `curl -H "Authorization: Bearer {id}|{secret}" localhost:8001/api/user`.

## Known deviations from the Laravel API

- **`/user` payload** returns the meaningful, non-secret user attributes plus
  `photo_url`, `roles`, `permissions`. Deprecated/secret columns present in the
  Laravel `users.toArray()` (e.g. `two_factor_secret`) are intentionally omitted.
- **PIF PDF** is a functional port of `pif.blade.php` (header, pax, flights,
  hotels, mutawifs, tour leaders, handlers, manasik, itinerary, notes) rendered
  via Gotenberg — not a pixel-identical reproduction. On a render error the
  `pdf_data`/`pdf_name` fields are returned empty rather than failing the request.
- The small **customer API** (`routes/api_customer.php`) is out of scope.
