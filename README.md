# Jungo

A minimal Go web application skeleton built on [Gin](https://github.com/gin-gonic/gin) and
[Uber Fx](https://github.com/uber-go/fx), backed by a standalone, reusable library of
infrastructure packages, [`junkit`](https://github.com/jungo-dev/junkit) — a public module pulled
in as a normal versioned dependency in `go.mod` (not a local sibling checkout).

**New here?** Follow sections 1 → 3 to get the API running and make your first request (about 10
minutes). Read the rest when you need it — Part II explains how the code is organized, Part III is
a reference for the `junkit` packages.

## Table of contents

**Part I — Get it running**

1. [Install the required tools](#1-install-the-required-tools)
2. [Quick start](#2-quick-start)
3. [Try the API](#3-try-the-api)
4. [Everyday commands](#4-everyday-commands)
5. [Configuration](#5-configuration)
6. [Authentication](#6-authentication)
7. [Live debug dashboard](#7-live-debug-dashboard)
8. [Console commands](#8-console-commands)

**Part II — Understand and extend the code**

9. [Mental model](#9-mental-model)
10. [Project layout](#10-project-layout)
11. [How a request flows through the app](#11-how-a-request-flows-through-the-app)
12. [Anatomy of the sample feature (`user`)](#12-anatomy-of-the-sample-feature-user)
13. [Adding a new feature](#13-adding-a-new-feature)
14. [Adding a new console command](#14-adding-a-new-console-command)

**Part III — `junkit` reference**

15. [Core vs. optional packages](#15-core-vs-optional-packages)
16. [Package tour](#16-package-tour)
17. [Usage examples](#17-usage-examples)

---

# Part I — Get it running

## 1. Install the required tools

**This project will not run without these installed first.**

| Tool | Needed for | macOS | Windows |
|---|---|---|---|
| **Docker Desktop** (Docker + Docker Compose) | Running the app, Postgres, and every other service — the primary, supported way to run this project | `brew install --cask docker`, or download from [docker.com](https://www.docker.com/products/docker-desktop/) | Download from [docker.com](https://www.docker.com/products/docker-desktop/) (the installer sets up the required WSL2 backend), or `winget install Docker.DockerDesktop` |
| **[`golang-migrate`](https://github.com/golang-migrate/migrate) CLI** | `make migrate-*` — applying database migrations. It always runs from your host, pointed at the Postgres port the dev/prod stack exposes | `brew install golang-migrate` | `scoop install migrate` (needs [Scoop](https://scoop.sh) — see below) |
| **`psql`** (PostgreSQL client) | `make migrate-up` / `make migrate-functions` / `make db-seed` — applying the SQL functions and demo seeders. Like `migrate`, it runs from your host against the exposed Postgres port | `brew install libpq && brew link --force libpq` (client only, no server) | `scoop install postgresql` (includes `psql`) |
| **[`sqlc`](https://sqlc.dev)** *(optional)* | `make sqlc` — regenerating `internal/database/sqlc` after you change a query. Not needed just to run the app | `brew install sqlc` | Download the Windows binary from the [sqlc releases page](https://github.com/sqlc-dev/sqlc/releases) |
| **Go 1.26.5** *(optional)* | Only if you run things outside Docker (`go build`, `go test`, `go run ./cmd/console`, ...) | `brew install go` | [go.dev/dl](https://go.dev/dl/) |

Windows without [Scoop](https://scoop.sh) yet: install it first, in a regular (non-admin)
PowerShell terminal, then run `scoop install migrate postgresql` as above.

```powershell
Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser
Invoke-RestMethod -Uri https://get.scoop.sh | Invoke-Expression
```

Not sure everything is installed? `make app-init` (next step) re-checks whether `migrate`,
`psql` and `sqlc` are on your `PATH` and prints the exact install command if any is missing.

---

## 2. Quick start

`junkit` is a public module, so the Docker build fetches it straight from the Go module proxy —
no extra checkout needed.

```bash
make app-init              # interactive: app name, db name/user/pass, ports — creates .env
make app-dev-bg            # builds and starts App + PostgreSQL, detached
make migrate-up            # applies migrations; in development also seeds demo accounts
```

That's it. The API is now listening on `http://localhost:<API_SERVER_PORT>` (`8080` unless you
changed it in `make app-init`).

Hot reload is enabled in dev (via [Air](https://github.com/air-verse/air)): edit any `.go` file
in this repo and the app restarts automatically.

---

## 3. Try the API

```bash
# 1. health check
curl http://localhost:<API_SERVER_PORT>/health

# 2. log in with a seeded demo account — copy "access_token" from the response
curl -X POST http://localhost:<API_SERVER_PORT>/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "admin@jungo.com",
    "password": "Password@123"
  }'

# 3. call a protected route with that access token
curl http://localhost:<API_SERVER_PORT>/api/v1/users \
  -H "Authorization: Bearer <access_token>"
```

Every `/api/v1/users` route requires a logged-in user — see [6 Authentication](#6-authentication)
for the full login / refresh / logout flow.

### Demo accounts

Seeded by `make migrate-up` **only when `API_ENVIRONMENT=development`**. All use the password
`Password@123`:

- `admin@jungo.com`
- `manager@jungo.com`
- `member@jungo.com`
- `user@jungo.com`

They are plain users for now — the names only anticipate future roles. Re-seed any time with
`make db-seed` (existing emails are skipped).

Outside development, create the first user with:

```bash
make console CMD="user:create" ARGS="-email=... -first-name=... -last-name=..."
```

---

## 4. Everyday commands

Run `make help` for the full list.

**Running the app**

| Command | What it does |
|---|---|
| `make app-init` | Interactively create `.env` for a fresh clone (app name, db, ports) |
| `make app-dev-bg` | Build and start the dev stack (App + PostgreSQL), detached |
| `make app-dev` | Same, but attached (streams logs, Ctrl+C to stop) |
| `make app-dev WITH=cache` | Also start Redis and enable the `cache` package |
| `make app-dev WITH=full` | Start every optional service (Redis, Mailpit) |
| `make app-logs` | Follow the app container's logs |
| `make app-dev-down` | Stop and remove the dev stack |
| `make app-prod` / `make app-prod-bg` / `make app-prod-down` | Same as above, for the prod stack |

**Database**

| Command | What it does |
|---|---|
| `make migrate-up` | Apply pending migrations + SQL functions (+ demo data in development) |
| `make migrate-down` | Roll back the last migration |
| `make migrate-refresh` | Drop everything and re-run all migrations (**destroys data**) |
| `make migrate-create NAME=add_x` | Scaffold a new migration |
| `make db-seed` | Apply `internal/database/seeders/*.sql` (demo data, never in production) |
| `make sqlc` | Regenerate `internal/database/sqlc` from `internal/database/queries` |

**Code generation & tooling**

| Command | What it does |
|---|---|
| `make feature NAME=product` | Generate a new feature — see [13](#13-adding-a-new-feature) |
| `make feature-remove NAME=product` | Remove a generated feature |
| `make command NAME=... SIGNATURE=...` | Generate a new CLI command — see [14](#14-adding-a-new-console-command) |
| `make console CMD="user:list"` | Run a CLI command inside the running stack — see [8](#8-console-commands) |
| `make test` / `make vet` / `make fmt` / `make tidy` | Standard Go checks |

### Updating `junkit`

`junkit` is fetched as a pinned dependency at build time, not mounted live — hot reload only
watches this repo. To pick up a `junkit` change: publish a new tag in that repo, bump the version
here (`go get github.com/jungo-dev/junkit@vX.Y.Z`), and rebuild.

---

## 5. Configuration

All configuration is environment variables in `.env` (created by `make app-init`), parsed into
`internal/config/config.go`'s `Config` struct via [`caarlos0/env`](https://github.com/caarlos0/env).
`.env.example` documents every variable inline. Highlights:

| Variable | Purpose |
|---|---|
| `API_ENVIRONMENT` | `development` enables demo seeding and dev-friendly defaults |
| `API_SERVER_PORT` | Port the API listens on (default `8080`) |
| `DB_*` | Postgres connection + pool tuning |
| `AUTH_*` | Login, tokens and brute-force limits — see [Authentication › Configuration](#authentication-configuration) |
| `TRACER_DEBUG_KEY` / `TRACER_DEBUG_VALUE` | Query-param name/value that unlocks the [debug dashboard](#7-live-debug-dashboard) |
| `CACHE_ENABLED` | `cache` package is a no-op unless this is `true` (and Redis is started with `WITH=cache`) |
| `STORAGE_BASE_DIR` / `STORAGE_BASE_URL` | Where uploaded avatars are stored and served from |
| `TELEGRAM_BOT_TOKEN` / `TELEGRAM_CHAT_ID` | Ops alerts for panics/rate-limit breaches; empty = safe no-op |
| `RECAPTCHA_MOCK` | Verifier always succeeds when true (default outside production) |

### Container resource limits

Three more variables aren't part of `Config` — they're read directly by
`deploy/docker-compose.dev.yaml` / `docker-compose.yaml` to cap the `app` container's resources.
Both files fall back to sane defaults, so you only need these to override:

| Variable | Purpose | Dev default | Prod default |
|---|---|---|---|
| `APP_MEM_LIMIT` | Hard memory cap on the `app` container | `1024M` | `2048M` |
| `APP_CPU_LIMIT` | Hard CPU cap on the `app` container | `1.0` | `2.0` |
| `APP_GOMEMLIMIT` | Go's soft memory limit ([`GOMEMLIMIT`](https://pkg.go.dev/runtime#hdr-Environment_Variables)) — lets the GC back off before hitting the hard cap above instead of getting OOM-killed. Keep it ~90% of `APP_MEM_LIMIT` | `900MiB` | `1800MiB` |

Lower-spec dev machines rarely need to touch these; raise them if the first build or hot-reload
feels CPU/memory-starved.

---

## 6. Authentication

`internal/features/auth` logs users in with email + password and issues two tokens:

| Token | Lifetime | Used for |
|---|---|---|
| **Access token** | 15 minutes | Every protected request: `Authorization: Bearer <access_token>` |
| **Refresh token** | 30 days | Only `POST /auth/refresh`, to get a new pair when the access token expires |

Both belong to one **session** (one login on one device). Tokens are opaque strings (not JWTs):
only their hash is stored, so a leaked database does not leak usable tokens.

### Typical flow

```
login ──► access + refresh token
  │
  ├─► call APIs with the access token
  │
  ├─► 401 token_expired? ──► POST /auth/refresh with the refresh token ──► new pair
  │
  └─► logout
```

### Endpoints

| Method | Path | Needs | Purpose |
|---|---|---|---|
| `POST` | `/api/v1/auth/login` | — | Log in, get a token pair |
| `POST` | `/api/v1/auth/refresh` | refresh token (body) | Exchange the refresh token for a new pair |
| `GET` | `/api/v1/auth/me` | access token | Current user + session info |
| `GET` | `/api/v1/auth/sessions` | access token | Devices where the user is logged in (`current: true` = this one) |
| `POST` | `/api/v1/auth/logout` | access token | Log out this device |
| `POST` | `/api/v1/auth/logout-all` | access token | Log out every device |
| `POST` | `/api/internal/auth/token-details` | `X-Internal-Secret` | Inspect any token (service-to-service) |
| `POST` | `/api/internal/auth/revoke` | `X-Internal-Secret` | End the session a token belongs to |

### Login

Send `X-Device-ID` (a stable id per app install/browser) if you can: logging in again from the
same device reuses its session instead of creating a new one. Without it, IP + User-Agent is used.

```bash
curl -X POST http://localhost:<API_SERVER_PORT>/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -H "X-Device-ID: iphone-7f3a" \
  -d '{
    "email": "admin@jungo.com",
    "password": "Password@123"
  }'
```

Response:

```json
{
  "status": "success",
  "message": "Logged in successfully",
  "data": {
    "access_token": "v1.…",
    "refresh_token": "v1.…",
    "token_type": "Bearer",
    "expires_in": 899,
    "refresh_expires_in": 2591999
  }
}
```

### Call a protected API

```bash
curl http://localhost:<API_SERVER_PORT>/api/v1/auth/me \
  -H "Authorization: Bearer <access_token>"
```

### Refresh

The refresh token goes in the **body**, never in `Authorization`. Each refresh token works
**once**; always store the new pair. Reusing an old one ends the session (possible theft).

```bash
curl -X POST http://localhost:<API_SERVER_PORT>/api/v1/auth/refresh \
  -H "Content-Type: application/json" \
  -d '{
    "refresh_token": "<refresh_token>"
  }'
```

### Logout

```bash
curl -X POST http://localhost:<API_SERVER_PORT>/api/v1/auth/logout \
  -H "Authorization: Bearer <access_token>"
```

### Errors clients should handle

Errors look like this:

```json
{
  "status": "error",
  "code": "UNAUTHORIZED",
  "message": "Token has expired"
}
```

| HTTP | `message` | What to do |
|---|---|---|
| 401 | Token has expired | Call `/auth/refresh`, then retry the request |
| 401 | Token has been revoked · Refresh token was already used… · Invalid token | Send the user to the login screen |
| 401 | Invalid token type | A refresh token was sent as `Bearer`; use the access token |
| 401 | Invalid email or password (…) | Wrong credentials; the longer variant means the IP is close to being blocked |
| 403 | Your account is not active | Account disabled |
| 429 | Too many failed login attempts… · Your IP has been temporarily blocked… | Too many failed logins; wait |

### Internal endpoints (service-to-service)

For other backend services, never browsers. They need the `X-Internal-Secret` header
(`AUTH_INTERNAL_SECRET`); when it is unset they answer `403` to everyone.

**Token details** — inspect any token, including expired or revoked ones (e.g. another service
checking who a token belongs to, or support investigating a session):

```bash
curl -X POST http://localhost:<API_SERVER_PORT>/api/internal/auth/token-details \
  -H "Content-Type: application/json" \
  -H "X-Internal-Secret: <AUTH_INTERNAL_SECRET>" \
  -d '{
    "token": "<access_or_refresh_token>"
  }'
```

Response (`token_status` is `active`, `expired` or `revoked`):

```json
{
  "status": "success",
  "message": "Token details retrieved successfully",
  "data": {
    "token_type": "access_token",
    "token_status": "active",
    "user": {
      "uuid": "5b1e…",
      "email": "admin@jungo.com",
      "first_name": "Admin",
      "last_name": "Jungo",
      "avatar_url": null,
      "active": true
    },
    "session": {
      "family_uuid": "3f0c…",
      "ip_address": "113.161.10.5",
      "user_agent": "Mozilla/5.0 (…)",
      "device_id": "iphone-7f3a",
      "created_at": "2026-10-01T08:00:00Z"
    },
    "access_token": {
      "uuid": "9d2a…",
      "expires_at": "2026-10-03T09:30:00Z",
      "status": "active"
    },
    "refresh_token": {
      "uuid": "c47e…",
      "expires_at": "2026-11-02T09:15:00Z",
      "status": "active"
    }
  }
}
```

**Revoke** — end the whole session a token belongs to (e.g. an admin tool forcing a device out):

```bash
curl -X POST http://localhost:<API_SERVER_PORT>/api/internal/auth/revoke \
  -H "Content-Type: application/json" \
  -H "X-Internal-Secret: <AUTH_INTERNAL_SECRET>" \
  -d '{
    "token": "<access_or_refresh_token>"
  }'
```

```json
{
  "status": "success",
  "message": "Token revoked successfully"
}
```

<a id="authentication-configuration"></a>
### Configuration

`make app-init` generates the three secrets. Everything else has sensible defaults.

| Variable | Default | Purpose |
|---|---|---|
| `AUTH_TOKEN_HMAC_SECRET` | — | Signs tokens. Base64, 32 bytes. Required outside development |
| `AUTH_TOKEN_ENCRYPTION_KEY` | — | Encrypts tokens. Base64, 32 bytes. Required outside development |
| `AUTH_INTERNAL_SECRET` | — | `X-Internal-Secret` for `/api/internal/auth/*`; empty disables them |
| `AUTH_ACCESS_TOKEN_TTL` | `15m` | Access token lifetime |
| `AUTH_REFRESH_TOKEN_TTL` | `720h` | Refresh token lifetime (how long a user stays logged in without activity) |
| `AUTH_SESSION_CACHE_TTL` | `3m` | How long a session is cached (Redis only); `0` disables the cache |
| `AUTH_REVOKED_RETENTION` | `168h` | How long expired/revoked tokens are kept before cleanup (needed to detect reused refresh tokens) |
| `AUTH_PASSWORD_COST` | `12` | bcrypt cost; +1 doubles the hashing time (12 ≈ 180 ms) |
| `AUTH_RECAPTCHA_ON_LOGIN` | `false` | Require a reCAPTCHA token on login (`recaptcha_token` in the body) |
| `AUTH_LOGIN_MAX_ATTEMPTS` / `AUTH_LOGIN_BLOCK_WINDOW` | `5` / `15m` | Failed logins per IP before a short block |
| `AUTH_LOGIN_BLACKLIST_ATTEMPTS` / `AUTH_LOGIN_BLACKLIST_WINDOW` | `8` / `24h` | Failed logins per IP before a long block (+ Telegram alert if configured) |
| `AUTH_LOGIN_EMAIL_MAX_ATTEMPTS` | `10` | Failed logins per email (any IP) within `AUTH_LOGIN_BLOCK_WINDOW` |

### Operations

- **First user**: seeded in development (see [Demo accounts](#demo-accounts)); elsewhere
  `make console CMD="user:create" ARGS="-email=... -first-name=... -last-name=..."`.
- **Cleanup**: run `make console CMD="auth:cleanup-tokens"` daily (cron) to delete old tokens.
- **Leaked key**: replace both `AUTH_TOKEN_*` keys and restart — every user must log in again.
- **Several app instances**: enable Redis (`CACHE_ENABLED=true`) so logout and login limits are shared.
- **Disabling or deleting a user** logs them out of every device immediately.

To protect your own routes with this auth, see step 5 of [Adding a new feature](#13-adding-a-new-feature).

---

## 7. Live debug dashboard

Append `?t_debug=<TRACER_DEBUG_VALUE>` (see `TRACER_DEBUG_KEY` / `TRACER_DEBUG_VALUE` in your
`.env` — `app-init` randomizes this per clone) to any request to get a pretty-printed debug
dashboard instead of the normal JSON response:

- request/server info,
- every SQL statement executed (with bound parameters) and its actual result rows,
- a timeline (waterfall) view breaking the request down into `logic` / `external` / `db` spans
  with their percentage of total time.

```bash
curl "http://localhost:<API_SERVER_PORT>/api/v1/users/<uuid>?t_debug=<TRACER_DEBUG_VALUE>"
```

> ⚠️ Never enable this in production — set `TRACER_DEBUG_VALUE` empty to disable the dashboard
> entirely.

This is powered by the `tracer` package — see [the `tracer` example](#tracer) for how to add your
own annotations and spans to the timeline.

---

## 8. Console commands

Besides the HTTP API, `cmd/console` is a second entrypoint for one-off CLI commands (health
checks, data backfills, ad-hoc reports) that need the same dependencies (database, cache,
services, ...) as the API server, without going through HTTP.

```bash
make console CMD="health:check"
make console CMD="user:list"
make console CMD="user:create" ARGS="-email=... -first-name=... -last-name=..."
make console CMD="auth:cleanup-tokens"
go run ./cmd/console health:check   # equivalent, run directly on the host
```

Running with no command lists every registered signature. To write your own, see
[14 Adding a new console command](#14-adding-a-new-console-command).

---

# Part II — Understand and extend the code

## 9. Mental model

```
junkit    → the reusable library of infrastructure packages
jungo     → the application that assembles them into a runnable API
```

- **`junkit`** packages know nothing about each other beyond a few explicit extension points
  (see [15 Core vs. optional](#15-core-vs-optional-packages)). Each one compiles and is useful on
  its own, and is documented via GoDoc comments in its own source — this app is what actually
  wires them together.
- **`jungo`** (this module) is the "composition root": it reads environment variables
  into a `Config` struct, turns that into each `junkit` package's `Options`, and hands everything
  to Fx to wire together. Adding a feature means writing a small, self-contained module and
  registering it — nothing else needs to change.

The `user` feature under `internal/features/user` isn't meant to be a real product feature — it's
a worked example. Copy its shape when you build your own features.

---

## 10. Project layout

```
jungo/
├── cmd/
│   ├── api/             HTTP API entrypoint (main.go)
│   ├── console/         CLI commands entrypoint — see 8
│   └── scaffold/        code generator behind `make feature` / `make command`
├── internal/
│   ├── app/             Fx composition root (fx.go, app.go)
│   ├── common/          shared helpers for the whole project
│   ├── config/          env-var → Config struct
│   ├── console/         console command kernel (Command interface, dispatcher, global commands)
│   ├── router/          global middleware + route collection
│   ├── database/        migrations, functions, seeders, sqlc queries + generated code
│   └── features/        user (sample feature — see 12), auth (see 6)
└── deploy/              Dockerfile, docker-compose (dev + prod)
```

The public [`junkit`](https://github.com/jungo-dev/junkit) module (a normal `go.mod`
dependency, no local checkout needed) holds every infrastructure package listed in
[16](#16-package-tour).

---

## 11. How a request flows through the app

```
cmd/api/main.go
  → app.NewFx()                         builds the Fx graph, cfg := config.NewConfig()
      → internal/app/fx.go              GetFxOptions(): every junkit Module + feature Module
          → router.RegisterAll          applies global middleware, mounts every feature's routes
              → <feature>/router        e.g. user's v1 router: group + auth middleware + handlers
                  → handler             parses/validates the request, calls the service
                      → service         business logic (hashing, avatar upload, etc.)
                          → repository  talks to Postgres via sqlc-generated queries
```

Global middleware order (outermost first — see `internal/router/router.go`):

```
TracerDebug → Security → Trace → CORS → Payload → Limiter → Recover → (your handler)
```

Each has a reason for its position (documented in code): `TracerDebug` must be outermost so it can
catch panics re-raised by `Recover` when running in debug mode; `Limiter` runs before `Recover` so
a rate-limited request never reaches handler code; `Recover` is last so it wraps every handler.

---

## 12. Anatomy of the sample feature (`user`)

`internal/features/user` is the template to copy for a new feature. Each layer has one job and
only depends on the layer below it:

```
internal/features/user/
├── domain/           entity + interfaces (UserRepository, UserService) — no Gin, no sqlc, no Fx
├── repository/       UserRepository impl — talks to Postgres via internal/database/sqlc
├── service/          UserService impl — business logic (bcrypt hashing, avatar upload via storage)
├── dto/v1/           request/response structs + converters to/from domain.User
├── handler/v1/       thin HTTP layer: bind → call service → respond
├── router/v1/        registers routes on a *gin.RouterGroup, applies middleware.BearerAuth
├── command/          feature-scoped console commands (e.g. user:list)
└── module.go         fx.Module wiring the above together + registerTranslations
```

Routes are collected automatically: any type implementing `router.Routes` (optionally also
`router.Versioned`) that's provided into the `"routes"` Fx group gets mounted by
`router.RegisterAll` — `internal/router` never imports feature packages directly.

The `auth` feature (login, tokens, sessions) follows the same shape and is documented from the
API side in [6 Authentication](#6-authentication).

---

## 13. Adding a new feature

Two ways, from fastest to most hands-on.

### Option A — generate it (recommended)

`cmd/scaffold` is a code generator (`junkit/scaffold` is the reusable rendering/Fx-registration
engine; `cmd/scaffold/templates.go` holds this app's actual templates, matching `user`'s layer
shape exactly):

```bash
make feature NAME=product              # or: TABLE=custom_table_name to override the default "products"
make migrate-up                        # creates the generated table
go build ./...
```

This creates:

- a migration (up/down),
- a `queries/product.sql` + hand-written `sqlc/product.sql.go` (same shape as `user.sql.go` —
  replace with a real `sqlc generate` once you've refined the schema),
- the full `domain/repository/service/dto/handler/router/module` layer set for a generic
  `name + status` entity,

then registers `product.Module` into `internal/app/fx.go` automatically. Adjust the generated
columns/fields to fit your actual domain, same as you'd hand-edit anything else.

`make feature-remove NAME=product` reverses everything *except* the migration files — dropping a
table is destructive, so that step is left for you to confirm explicitly (`make migrate-down`,
then delete the files).

### Option B — copy `user` by hand

When the generic template doesn't fit:

1. Copy the `user` directory structure, renaming `user` → your feature name throughout.
2. Add a migration: `make migrate-create NAME=add_<feature>_table`, then `make migrate-up`.
3. Write the SQL in `internal/database/queries/<feature>.sql` (sqlc-annotated) and either hand-write
   or `make sqlc` a `internal/database/sqlc/<feature>.sql.go`.
4. Wire the feature's `Module` into `internal/app/fx.go` next to `user.Module`.
5. If your routes need protecting, see below.

### Protecting your routes

Inject `middleware.Authenticator[*authdomain.Identity]` (provided by the auth module) into your
router and apply `middleware.BearerAuth`, same as `user_router.go` does:

```go
// router: require a valid access token on every route in this group
group.Use(middleware.BearerAuth(r.authenticator, r.responder, middleware.BearerAuthOptions{}))

// handler: who is calling?
identity := security.MustGetIdentity[*authdomain.Identity](c)
```

---

## 14. Adding a new console command

The console is an app-level mechanism (`internal/console`) — not to be confused with the
`junkit/console` package, which is just the colored `Successf`/`Infof`/`Warnf`/`Fatalf` output
helpers a command's `Run` prints through.

### Quickest: generate it

```bash
make command NAME=health_check SIGNATURE=health:check           # global command
make command NAME=list_users SIGNATURE=user:list FEATURE=user    # feature-scoped command
```

### By hand

1. Write a type implementing `console.Command`:

   ```go
   // internal/console/command.go
   type Command interface {
   	Signature() string                        // CLI name, e.g. "user:list"
   	Run(ctx context.Context, args []string) error
   }
   ```

   - **Global** commands (not tied to any one feature, e.g. `health:check`) go under
     `internal/console/commands/` — see
     [`health_check_command.go`](internal/console/commands/health_check_command.go).
   - **Feature-scoped** commands live inside the feature they operate on and share its services,
     under `internal/features/<feature>/command/` — e.g. `user:list`
     ([`list_users_command.go`](internal/features/user/command/list_users_command.go)) reuses the
     same `domain.UserService` the HTTP handler calls.

2. Give it a `namespace:verb` signature (e.g. `product:sync`) so it stays discoverable via the
   no-argument command listing.
3. Register it into the `"commands"` Fx group — in
   [`internal/console/commands/module.go`](internal/console/commands/module.go) for a global
   command, or in your feature's own `module.go` (next to its routes) for a feature-scoped one:

   ```go
   fx.Provide(
   	fx.Annotate(
   		NewListUsersCommand,
   		fx.As(new(console.Command)),
   		fx.ResultTags(`group:"commands"`),
   	),
   ),
   ```

4. Use `junkit/console`'s `Successf`/`Infof`/`Warnf`/`Fatalf` for terminal output inside `Run` —
   see the [`console` example](#console).

### How it's wired

`app.NewConsoleFx()` (`internal/app/fx.go`) builds a separate Fx app that reuses `CoreModules`
(the same DB/cache/services graph as the API) but swaps `HTTPModules` for:

```go
fx.Provide(console.Module),      // collects every Command into the "commands" Fx group
fx.Invoke(console.RunSelected),  // registers the OnStart hook that dispatches os.Args[1]
```

`console.RunSelected` (`internal/console/kernel.go`) reads `os.Args[1]` as the command's
signature, finds the matching `Command` among everything provided into the `"commands"` group,
calls its `Run`, then shuts the Fx app down.

---

# Part III — `junkit` reference

## 15. Core vs. optional packages

`internal/app/fx.go` groups `junkit` packages into two tiers:

- **Core** — `logger`, `database`, `response`, `i18n`, `validation`. The app doesn't build without
  these.
- **Optional** — `cache`, `storage`, `httpclient`, `telegram`, `notification`, `recaptcha`,
  `tracer`. Each is registered as its own `fx.Module` line; comment one out (and its `Options`
  provider) and the app still builds — see the comment block above the "OPTIONAL PACKAGES"
  section in `fx.go` for exactly which ones are safe to remove alone vs. together.

Two optional packages are wired to a real call site in this skeleton and are **not** actually
removable without further changes:

- `storage` — the `user` feature's avatar upload depends on it directly.
- `tracer` — wired into `database` through a `Decorate` hook (see below), and into
  `middleware.TracerDebug`.

### The `Decorate` hook (why `database` doesn't import `tracer`)

`junkit/database` must never import `junkit/tracer` (a Core package can't depend on an Optional
one), but the debug dashboard still needs to see every query. The fix is an extension point:
`database.Options.Decorate func(DBTX) DBTX`, which this app wires up:

```go
// internal/app/fx.go
func provideDatabaseOptions(cfg *config.Config) database.Options {
	return database.Options{
		DSN: cfg.DSN(),
		// ...
		Decorate: func(inner database.DBTX) database.DBTX {
			return tracer.NewDBWrapper(inner)
		},
	}
}
```

`tracer.NewDBWrapper` is a zero-overhead passthrough unless the current request's context carries
an enabled `tracer.Debugger` — so this is always safe to leave in place. The same pattern
(`optional:"true"` Fx tags + nil-checks) keeps `middleware`'s `Recover`/`Limiter` decoupled from
`notification`/`telegram`.

---

## 16. Package tour

Every package below is documented in full via GoDoc comments in its own source — this table is
just an index. Run `go doc github.com/jungo-dev/junkit/<package>` from this directory for the full
API.

| Package | Purpose |
|---|---|
| `console` | Colored CLI status output for bootstrap/tooling scripts (not app logging) |
| `logger` | zap-based structured logger, dual console/file output, trace-ID propagation |
| `pagination` | Parses list-endpoint query params (page/page_size/search/sort) into offset/limit — no Fx module, pure functions |
| `i18n` | Thread-safe message translator with per-language catalogs and fallback |
| `response` | Standard `{ data, error, meta }` JSON envelope, `Responder` interface, field include/omit filtering |
| `validation` | Wraps `go-playground/validator` with custom rules + localized, field-keyed error messages |
| `database` | Postgres pool (pgx/pgxpool), context-scoped transactions, Postgres error classification |
| `cache` | Generic `Cache[T]` interface with Redis / in-memory / no-op backends, chosen via config |
| `storage` | File upload/delete behind a `Service` interface (local-filesystem implementation shipped) |
| `httpclient` | Production-tuned `*http.Client` with optional retry + exponential backoff |
| `telegram` | Minimal Telegram Bot API client (text messages, document upload) |
| `notification` | Sends panic/rate-limit alerts to Telegram with request context and repro `curl` command |
| `recaptcha` | Google reCAPTCHA v3 verification, with a `Mock` verifier for local dev/tests |
| `tracer` | Request-scoped debug logger — powers the `?t_debug=` dashboard (SQL, results, request/server info) |
| `middleware` | Gin middleware: CORS, rate limiting, panic recovery, security headers, request tracing, body capture |
| `scaffold` | Code-generation engine (name-form derivation, templated file writing, Fx registration) behind `cmd/scaffold` — see [13](#13-adding-a-new-feature) |

**Convention:** every package except `pagination`, `middleware`, and `scaffold` exposes a `var
Module = fx.Module(...)` (or `fx.Provide(...)`) for one-line Fx registration — see how each is
registered in `internal/app/fx.go`. `pagination` needs no wiring (pure functions); `middleware` is
deliberately *not* centrally wired — because call sites need differently configured instances
(e.g. `Limiter` per route group), so its constructors are called directly wherever routes are
registered (see `internal/router/router.go`). `scaffold` is a build-time CLI dependency, not
something the running application imports at all.

---

## 17. Usage examples

One example per package — either lifted directly from this codebase (file path given) or, for
packages this skeleton doesn't call into anywhere, a minimal standalone snippet. These examples
are the "how do I actually call this" complement to the GoDoc reference.

### console

Not wired through Fx — call directly from CLI-style code (migration runners, code generators,
`main` bootstrap, console commands), never from request-handling code:

```go
console.Stepf("→", "applying migration %s", name)
console.Successf("migration %s applied", name)
console.Warnf("optional service unavailable: %v", err)
console.Fatalf("cannot connect to database: %v", err) // prints and os.Exit(1)
```

### logger

Injected wherever a constructor takes `*zap.Logger` (Fx resolves it from `logger.Module`). Inside
a request, attach the request's trace ID so every log line can be correlated
(`junkit/middleware/trace.go` does this for every request automatically):

```go
ctx = logger.WithTraceID(ctx, traceID)
// ... deeper in the call stack:
log.Info("creating user", zap.String("trace_id", logger.GetTraceID(ctx)), zap.String("email", input.Email))
```

### pagination

Real usage from [`internal/features/user/handler/v1/user_handler.go`](internal/features/user/handler/v1/user_handler.go):

```go
var req v1dto.ListUsersRequest
if err := ctx.ShouldBindQuery(&req); err != nil { /* ... */ }
req.SetDefaults("created_at", "desc")

users, total, err := h.service.GetUsers(ctx.Request.Context(), req.ToFilter())
meta := pagination.NewPagination(req.Page, req.PageSize, total)
h.responder.Pagination(ctx, http.StatusOK, "operation_successful", v1dto.NewUserResponseList(users), meta)
```

### i18n

Every feature registers its own message keys on the shared `*i18n.Translator` — real usage from
[`internal/features/user/module.go`](internal/features/user/module.go):

```go
func registerTranslations(translator *i18n.Translator) {
	translator.AddTranslations(map[string]map[string]string{
		i18n.LangEN: {
			"user_not_found": "User not found",
		},
	})
}
```

`response.Responder` and `validation.Validator` resolve message keys through the same translator
(see below) — direct lookups (e.g. outside a request) use `translator.GetMessage(key, lang)`.

### response

Real usage from `user_handler.go` — a `Responder` is injected into every handler and is the only
way handlers write to the HTTP response:

```go
// single resource
h.responder.SendWithData(ctx, http.StatusOK, "operation_successful", v1dto.NewUserResponse(user))

// list + pagination metadata
h.responder.Pagination(ctx, http.StatusOK, "operation_successful", users, meta)

// domain/validation error → correct HTTP status + localized message, automatically
h.responder.Error(ctx, err)
```

### validation

Real usage from `user_handler.go`, turning a bind/validate failure into a field-keyed, localized
error payload:

```go
if err := ctx.ShouldBindJSON(&req); err != nil {
	h.responder.SendWithData(ctx, http.StatusUnprocessableEntity, "validation_error",
		h.validator.GetValidationErrors(ctx, err))
	return
}
```

### database

Real usage from [`internal/features/user/repository/user_repository.go`](internal/features/user/repository/user_repository.go)
— `db.Executor(ctx)` returns a plain `DBTX` (pool, or the active transaction if one is on `ctx`
via `WithTransaction`), and `database.Match` maps a Postgres error to a domain error:

```go
func (r *UserRepository) GetByUUID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	row, err := sqlc.New(r.db.Executor(ctx)).GetUserByUUID(ctx, id)
	if err != nil {
		return nil, database.Match(err, map[database.ErrorType]error{
			database.ErrorNotFound: domain.ErrUserNotFound,
		})
	}
	return mapUser(row), nil
}
```

Wrapping several statements in one transaction:

```go
err := db.WithTransaction(ctx, func(ctx context.Context) error {
	// every call in here that does db.Executor(ctx) shares this one tx
	if err := repo.Create(ctx, input); err != nil {
		return err // rolls back
	}
	return otherRepo.Touch(ctx, id)
})
```

### storage

Real usage from [`internal/features/user/service/user_service.go`](internal/features/user/service/user_service.go)
(avatar upload/delete) — the service depends on the `storage.Service` interface, not a concrete
implementation:

```go
func NewUserService(repo domain.UserRepository, storageService storage.Service) *UserService {
	return &UserService{repo: repo, storage: storageService}
}

func (s *UserService) UploadAvatar(ctx context.Context, id uuid.UUID, file domain.AvatarFile) (*domain.User, error) {
	newURL, err := s.storage.UploadFile(ctx, file.Reader, file.Filename)
	// ...
}
```

### cache

Not called from this skeleton (no feature needs caching yet). This app still registers
`cache.Module`, which resolves a `cache.Cache[T]` — depend on the interface and let config decide
the backend (Redis, in-memory, or a no-op when `CACHE_ENABLED=false`):

```go
type UserService struct {
	users cache.Cache[*domain.User]
}

func (s *UserService) GetUser(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	return s.users.GetOrSet(ctx, "user:"+id.String(), 5*time.Minute, func() (*domain.User, error) {
		return s.repo.GetByUUID(ctx, id)
	})
}
```

### httpclient

This app provides a default client (`fx.Provide(httpclient.NewDefaultClient)`) that `telegram`
depends on. Use it directly, or with custom retry tuning, wherever you need to call another HTTP
service:

```go
client := httpclient.NewClient(httpclient.Options{MaxRetries: 3, Timeout: 5 * time.Second})
resp, err := client.Get("https://example.com/health")
```

### telegram + notification

Not called from this skeleton's routes directly, but wired into `junkit/middleware`'s `Recover`
and `Limiter` (registered in
[`internal/router/router.go`](internal/router/router.go)) to send ops alerts on panics and
rate-limit breaches:

```go
middleware.Recover(logger, notifier, responder, middleware.RecoverOptions{ /* ... */ })
```

`notifier` is `notification.Notifier`, itself built from a `telegram.Client` — send an alert
manually the same way:

```go
notifier.SendMessageHTML(ctx, "<b>Deploy finished</b>", "info")
```

### recaptcha

Not called from this skeleton (no form needs bot protection yet). Verify a token from a handler:

```go
if err := recaptchaClient.Verify(ctx, req.RecaptchaToken, ctx.ClientIP()); err != nil {
	responder.Error(ctx, err) // score too low / verification failed
	return
}
```

`RECAPTCHA_MOCK=true` (the dev default) swaps in a verifier that always succeeds, so local
development and tests never need a real Google token.

### tracer

Powers the [`?t_debug=` dashboard](#7-live-debug-dashboard) — the `database.Options.Decorate`
hook in `internal/app/fx.go` already captures every SQL query automatically. To add your own
breakpoint-style annotations visible in that same dashboard, call the package-level helpers
anywhere a `context.Context` (or `*gin.Context`) is available:

```go
tracer.C(ctx, "about to charge the customer")
tracer.V(ctx, "input", input)
if err != nil {
	tracer.E(ctx, "charge failed", err)
}
```

Outside an active debug request these calls fall back to structured `zap` logging via
`tracer.BindLogger`, so it's safe to leave instrumentation in place permanently.

**Timeline spans** — the dashboard also renders a waterfall/timeline of the request, broken down
by category (`tracer.CategoryLogic` default, `tracer.CategoryExternal`, `tracer.CategoryDB` — SQL
queries populate this one automatically). Time a block with `defer`:

```go
func (s *OrderService) Checkout(ctx context.Context, order Order) error {
	defer tracer.Span(ctx, "Calculate Discount & Taxes")()
	// ...
}
```

or time an inline closure with `tracer.Measure`:

```go
tracer.Measure(ctx, "Charge Payment Provider", func() {
	err = s.paymentClient.Charge(ctx, order.Total)
}, tracer.CategoryExternal)
```

Spans nest (call one inside another) and the dashboard shows each one's exclusive time and its
share of the total request time. Like `tracer.C`/`V`/`E`, both are no-ops outside an active debug
request, so it's safe to leave them in permanently.

### middleware

Real usage — the full global chain from
[`internal/router/router.go`](internal/router/router.go)'s `registerGlobalMiddlewares`:

```go
r.Router.Use(
	middleware.TracerDebug(cfg.TracerDebugKey, cfg.TracerDebugValue),
	middleware.Security(),
	middleware.Trace(middleware.TraceOptions{Version: cfg.Version, Environment: cfg.Environment}),
	middleware.CORS(middleware.CORSOptions{AllowOrigins: cfg.CORS.AllowOrigins}),
	middleware.Payload(middleware.PayloadOptions{MaxBodySize: cfg.Security.MaxRequestBodySize}, r.Responder),
	middleware.Limiter(middleware.LimiterOptions{RequestsPerSecond: 10, Burst: 30}, r.Logger, r.Responder, r.Notifier),
	middleware.Recover(r.Logger, r.Notifier, r.Responder, middleware.RecoverOptions{ /* ... */ }),
)
```

Each constructor takes its own `Options` (no shared Fx module) so a route group needing a
different rate limit can call `middleware.Limiter` again with different `LimiterOptions`, e.g.
directly inside a feature's router.
