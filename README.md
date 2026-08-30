# Jungo

A minimal Go web application skeleton built on [Gin](https://github.com/gin-gonic/gin) and
[Uber Fx](https://github.com/uber-go/fx), backed by a standalone, reusable library of
infrastructure packages, [`junkit`](https://github.com/jungo-dev/junkit) — a public module pulled
in as a normal versioned dependency in `go.mod` (not a local sibling checkout).

If you're new here, read this file top to bottom once, then use it as a map back into the code.

---

## 1. Mental model

```
junkit    → the reusable library of infrastructure packages
jungo     → the application that assembles them into a runnable API
```

- **`junkit`** packages know nothing about each other beyond a few explicit extension points
  (see [4 Core vs. Optional](#4-core-vs-optional-packages)). Each one compiles and is useful on
  its own, and is documented via GoDoc comments in its own source — this app is what actually
  wires them together.
- **`jungo`** (this module) is the "composition root": it reads environment variables
  into a `Config` struct, turns that into each `junkit` package's `Options`, and hands everything
  to Fx to wire together. Adding a feature means writing a small, self-contained module and
  registering it — nothing else needs to change.

The `user` feature under `internal/features/user` isn't meant to be a real product feature — it's
a worked example. Copy its shape when you build your own features.

---

## 2. Quick start

Requirements: Docker + Docker Compose. (Go 1.26.5 locally too, if you want to run things outside
Docker.)

`junkit` is a public module, so the Docker build fetches it straight from the Go module proxy

```bash
make app-init              # interactive: app name, db name/user/pass, ports — creates .env
make app-dev-bg            # builds and starts App + PostgreSQL, detached
make migrate-up            # applies internal/database/migrations
```

The API is now listening on `http://localhost:<API_SERVER_PORT>` (`8080` unless you changed it in
`make app-init`).

```bash
# health check
curl http://localhost:<API_SERVER_PORT>/health

# create a user (replace <API_KEY> with the value app-init generated in your .env)
curl -X POST http://localhost:<API_SERVER_PORT>/api/v1/users \
  -H "Authorization: Bearer <API_KEY>" \
  -H "Content-Type: application/json" \
  -d '{"email":"jane@example.com","password":"Secret@12345","first_name":"Jane","last_name":"Doe"}'
```

Other useful commands (see `Makefile` for the full list, or run `make help`):

| Command | What it does |
|---|---|
| `make app-init` | Interactively create `.env` for a fresh clone (app name, db, ports) |
| `make app-dev` | Same as above, but attached (streams logs, Ctrl+C to stop) |
| `make app-dev WITH=cache` | Also start Redis and enable the `cache` package |
| `make app-dev WITH=full` | Start every optional service (Redis, Mailpit) |
| `make app-logs` | Follow the app container's logs |
| `make app-dev-down` | Stop and remove the dev stack |
| `make console CMD="user:list"` | Run a registered CLI command inside the running stack — see [11 Console commands](#11-console-commands) |
| `make migrate-create NAME=add_x` | Scaffold a new migration |
| `make sqlc` | Regenerate `internal/database/sqlc` from `internal/database/queries` |
| `make test` / `make vet` / `make fmt` | Standard Go checks |

Hot reload is enabled in dev (via [Air](https://github.com/air-verse/air)) and watches this
module's source. `junkit` is fetched as a pinned dependency at build time, not mounted live — to
pick up a `junkit` change, publish a new tag in that repo, bump the version in `go.mod`
(`go get github.com/jungo-dev/junkit@vX.Y.Z`), and rebuild.

### Live debug dashboard

Append `?t_debug=<TRACER_DEBUG_VALUE>` (see `TRACER_DEBUG_KEY` / `TRACER_DEBUG_VALUE` in your
`.env` — `app-init` randomizes this per clone) to any request to get a pretty-printed debug
dashboard instead of the normal JSON response: request/server info, every SQL statement executed
(with bound parameters), its actual result rows, and a timeline (waterfall) view breaking the
request down into `logic` / `external` / `db` spans with their percentage of total time. This is
the `tracer` package — see [6](#6-package-tour-junkit) for the package and [7](#tracer) for how to
add your own spans to the timeline.

```bash
curl "http://localhost:<API_SERVER_PORT>/api/v1/users/<uuid>?t_debug=<TRACER_DEBUG_VALUE>"
```

Never enable this in production — set `TRACER_DEBUG_VALUE` empty to disable the dashboard
entirely.

---

## 3. How a request flows through the app

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

## 4. Core vs. optional packages

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

## 5. Configuration

All configuration is environment variables, parsed into `internal/config/config.go`'s `Config`
struct via [`caarlos0/env`](https://github.com/caarlos0/env). Start from `.env.example` — it
documents every variable inline. Highlights:

| Variable | Purpose |
|---|---|
| `API_KEY` | Static Bearer token required by `Authorization: Bearer <key>` on the `user` routes |
| `TRACER_DEBUG_KEY` / `TRACER_DEBUG_VALUE` | Query-param name/value that unlocks the debug dashboard (2) |
| `DB_*` | Postgres connection + pool tuning |
| `CACHE_ENABLED` | `cache` package is a no-op unless this is `true` (and Redis is started with `WITH=cache`) |
| `STORAGE_BASE_DIR` / `STORAGE_BASE_URL` | Where uploaded avatars are stored and served from |
| `TELEGRAM_BOT_TOKEN` / `TELEGRAM_CHAT_ID` | Ops alerts for panics/rate-limit breaches; empty = safe no-op |
| `RECAPTCHA_MOCK` | Verifier always succeeds when true (default outside production) |

---

## 6. Package tour (`junkit`)

Every package below is documented in full via GoDoc comments in its own source (`../junkit/<pkg>`)
— this table is just an index. Run `go doc github.com/jungo-dev/junkit/<package>` from this
directory (the `replace` directive in `go.mod` resolves it to the sibling module) for the full
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
| `scaffold` | Code-generation engine (name-form derivation, templated file writing, Fx registration) behind `cmd/scaffold` — see 8 |

**Convention:** every package except `pagination`, `middleware`, and `scaffold` exposes a `var
Module = fx.Module(...)` (or `fx.Provide(...)`) for one-line Fx registration — see how each is
registered in `internal/app/fx.go`. `pagination` needs no wiring (pure functions); `middleware` is
deliberately *not* centrally wired — because call sites need differently configured instances
(e.g. `Limiter` per route group), so its constructors are called directly wherever routes are
registered (see `internal/router/router.go`). `scaffold` is a build-time CLI dependency, not
something the running application imports at all.

---

## 7. Usage examples

One example per package — either lifted directly from this codebase (file path given) or, for
packages this skeleton doesn't call into anywhere, a minimal standalone snippet. Every package
also has a full GoDoc comment in its own source; these examples are the "how do I actually call
this" complement to that reference.

### console

Not wired through Fx — call directly from CLI-style code (migration runners, code generators,
`main` bootstrap), never from request-handling code:

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

Powers the `?t_debug=` dashboard (2) — the `database.Options.Decorate` hook in
`internal/app/fx.go` already captures every SQL query automatically. To add your own
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

---

## 8. Anatomy of the sample feature (`user`)

`internal/features/user` is the template to copy for a new feature. Each layer has one job and
only depends on the layer below it:

```
internal/features/user/
├── domain/           entity + interfaces (UserRepository, UserService) — no Gin, no sqlc, no Fx
├── repository/       UserRepository impl — talks to Postgres via internal/database/sqlc
├── service/          UserService impl — business logic (bcrypt hashing, avatar upload via storage)
├── dto/v1/            request/response structs + converters to/from domain.User
├── handler/v1/        thin HTTP layer: bind → call service → respond
├── router/v1/          registers routes on a *gin.RouterGroup, applies the Auth middleware
└── module.go          fx.Module wiring the above together + registerTranslations
```

Routes are collected automatically: any type implementing `router.Routes` (optionally also
`router.Versioned`) that's provided into the `"routes"` Fx group gets mounted by
`router.RegisterAll` — `internal/router` never imports feature packages directly.

### Adding a new feature

Two ways to get a new feature scaffolded, from fastest to most hands-on:

**Generate it** with `cmd/scaffold` — a code generator (`junkit/scaffold` is the reusable
rendering/Fx-registration engine; `cmd/scaffold/templates.go` holds this app's actual templates,
matching `user`'s layer shape exactly):

```bash
make feature NAME=product              # or: TABLE=custom_table_name to override the default "products"
make migrate-up                        # creates the generated table
go build ./...
```

This creates a migration (up/down), a `queries/product.sql` + hand-written
`sqlc/product.sql.go` (same shape as `user.sql.go` — replace with a real `sqlc generate` once
you've refined the schema), and the full `domain/repository/service/dto/handler/router/module`
layer set for a generic `name + status` entity — then registers `product.Module` into
`internal/app/fx.go` automatically. Adjust the generated columns/fields to fit your actual domain,
same as you'd hand-edit anything else.

`make feature-remove NAME=product` reverses everything *except* the migration files — dropping a
table is destructive, so that step is left for you to confirm explicitly (`make migrate-down`,
then delete the files).

**Or copy `user` by hand**, when the generic template doesn't fit:

1. Copy the `user` directory structure, renaming `user` → your feature name throughout.
2. Add a migration: `make migrate-create NAME=add_<feature>_table`, then `make migrate-up`.
3. Write the SQL in `internal/database/queries/<feature>.sql` (sqlc-annotated) and either hand-write
   or `make sqlc` a `internal/database/sqlc/<feature>.sql.go`.
4. Wire the feature's `Module` into `internal/app/fx.go` next to `user.Module`.
5. If your routes need protecting, apply `middleware.Auth(cfg.APIKey, responder)` in your
   feature's router, same as `user_router.go` does.

### Auth

Routes under `user` are protected by `internal/middleware/auth.go` — a static, shared-secret
Bearer token checked with `crypto/subtle.ConstantTimeCompare` (timing-safe). This is intentionally
simple (no sessions, no per-user tokens); swap in real auth (JWT, OAuth, sessions) by replacing
this one middleware without touching anything else.

---

## 9. Project layout

```
jungo/
├── cmd/api/             entrypoint (main.go)
├── internal/
│   ├── app/             Fx composition root (fx.go, app.go)
│   ├── config/          env-var → Config struct
│   ├── router/          global middleware + route collection
│   ├── middleware/      app-specific middleware (currently just Auth)
│   ├── database/        migrations, sqlc queries + generated code
│   └── features/user/   sample feature — see 8
└── deploy/              Dockerfile, docker-compose (dev + prod)
```

The public [`junkit`](https://github.com/jungo-dev/junkit) module (a normal `go.mod`
dependency, no local checkout needed) holds every infrastructure package listed in 6.

---

## 10. Requirements

- Go **1.26.5**
- Docker + Docker Compose (recommended path — handles Postgres, hot reload, etc. for you)
- [`golang-migrate`](https://github.com/golang-migrate/migrate) CLI, only if running `make
  migrate-*` from your host instead of inside the container
- [`sqlc`](https://sqlc.dev), only if regenerating `internal/database/sqlc` via `make sqlc`

`make app-init` checks whether `migrate` and `sqlc` are on your `PATH` and tells you the install
command if either is missing.

---

## 11. Console commands

Besides the HTTP API, `cmd/console` is a second entrypoint for one-off CLI commands (health
checks, data backfills, ad-hoc reports) that need the same Fx-wired dependencies (database, cache,
services, ...) as the API server, without going through HTTP. This is an app-level mechanism
(`internal/console`) — not to be confused with the `junkit/console` package (7), which is just the
colored `Successf`/`Infof`/`Warnf`/`Fatalf` output helpers a command's `Run` prints through.

```bash
make console CMD="health:check"
make console CMD="user:list"
go run ./cmd/console health:check   # equivalent, run directly on the host
```

### How it's wired

`app.NewConsoleFx()` (`internal/app/fx.go`) builds a separate Fx app that reuses `CoreModules`
(the same DB/cache/services graph as the API) but swaps `HTTPModules` for:

```go
fx.Provide(console.Module),      // collects every Command into the "commands" Fx group
fx.Invoke(console.RunSelected),  // registers the OnStart hook that dispatches os.Args[1]
```

`console.RunSelected` (`internal/console/kernel.go`) reads `os.Args[1]` as the command's
signature, finds the matching `Command` among everything provided into the `"commands"` group,
calls its `Run`, then shuts the Fx app down. Running with no arguments lists every registered
signature.

### The `Command` interface

```go
// internal/console/command.go
type Command interface {
	Signature() string                        // CLI name, e.g. "user:list"
	Run(ctx context.Context, args []string) error
}
```

### Two places to register a command

- **Global** commands — not tied to any one feature, e.g. `health:check`
  ([`internal/console/commands/health_check_command.go`](internal/console/commands/health_check_command.go))
  — registered in [`internal/console/commands/module.go`](internal/console/commands/module.go), which is
  included once in `CoreModules`.
- **Feature-scoped** commands — live inside the feature they operate on and share its services, e.g.
  `user:list` ([`internal/features/user/command/list_users_command.go`](internal/features/user/command/list_users_command.go))
  reuses the same `domain.UserService` the HTTP handler calls. Registered directly in the feature's
  own `module.go`, next to its routes/repository/service providers — see
  [`internal/features/user/module.go`](internal/features/user/module.go).

Both register into the same Fx group, just with an `fx.Annotate`:

```go
fx.Provide(
	fx.Annotate(
		NewListUsersCommand,
		fx.As(new(console.Command)),
		fx.ResultTags(`group:"commands"`),
	),
),
```

### Adding a new command

1. Write a type implementing `console.Command` — global ones go under
   `internal/console/commands/`, feature ones under `internal/features/<feature>/command/`.
2. Give it a `namespace:verb` signature (e.g. `product:sync`) so `make console CMD=...` stays
   discoverable via the no-argument command listing.
3. Register it into the `"commands"` group — in `commands.Module` for a global command, or in your
   feature's own `module.go` for a feature-scoped one (same pattern as its routes).
4. Use `junkit/console`'s `Successf`/`Infof`/`Warnf`/`Fatalf` for terminal output inside `Run` —
   see the `console` example in [7](#console).
