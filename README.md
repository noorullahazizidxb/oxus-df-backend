# oxus-df-backend

A small, complete **CRUD backend** for OXUS Digital Finance, written in Go and built
to the approved Technology Stack Specification. It manages **financing products**
(create, list, view, edit, delete, export to Excel) behind an admin login.

> Every code file starts with a comment saying what the file is for, and every
> function and type has a comment saying what it does.

## Quick start (5 minutes)

```bash
cp .env.example .env     # 1. settings (defaults work locally)
make bootstrap           # 2. one time: install tools, generate code, create go.sum
make up                  # 3. start PostgreSQL, Valkey, API, worker and Nginx in Docker
# 4. open docs/api.http in VS Code -> click "Send Request" on "login", then any other request
```

The API is at **http://localhost:8080**. Details for each step are below.

---

## 1. Go's package manager (Go Modules) in 2 minutes

Go has **no separate package manager** (no npm). It is built into the `go` command and
is called **Go Modules**.

| File / folder | What it is | npm equivalent |
|---|---|---|
| `go.mod` | Project name, Go version and the list of libraries we use | `package.json` |
| `go.sum` | Security checksums of every downloaded library (commit it!) | `package-lock.json` |
| `~/go/pkg/mod` | Shared download cache on your computer | `node_modules` (but shared, not per project) |

The commands you will actually use:

| Command | What it does |
|---|---|
| `go mod tidy` | Downloads missing libraries, removes unused ones, updates `go.mod` and `go.sum` |
| `go get github.com/gin-gonic/gin@v1.10.0` | Adds or upgrades one library to a specific version |
| `go mod download` | Downloads everything listed in `go.mod` |
| `go mod verify` | Checks downloaded libraries were not tampered with |
| `go list -m all` | Shows every library and its version |
| `go install tool@latest` | Installs a command-line tool (we use this in `make tools`) |

Good to know: you do not run "install" by hand. When you build or test, Go downloads
what is missing. In this project run `make bootstrap` once and it does everything.

### Go vocabulary for a JavaScript / TypeScript developer

| Go term | What it means here | Closest JS/TS idea | Important difference |
|---|---|---|---|
| package | A directory of related `.go` files with the same `package` name | module folder | Files in one package share unexported names; there is no per-file export list. |
| `struct` | A typed data record, such as `products.Product` | `interface` + object/class fields | It has no inheritance and its zero value is usable. |
| method receiver | `func (s *Service) Create(...)` attaches `Create` to `Service` | class method | `*Service` is a pointer receiver: it avoids copying and can mutate the value if needed. |
| interface | A list of required methods, such as `products.Repository` | interface / abstract type | It is satisfied **implicitly**. The PostgreSQL repository and a test fake need no `implements` declaration. |
| `error` | The conventional failure result | `throw` / rejected Promise | Errors are returned explicitly and checked immediately: `if err != nil { return ..., err }`. |
| `context.Context` | Request lifetime, cancellation, deadlines, and request-scoped metadata | `AbortSignal` plus request locals | It is passed as the first parameter; do not store it in a long-lived struct. |
| `defer` | Schedule cleanup when the surrounding function returns | `finally` | It runs on every return path; this project uses it for connections, cancellation, and transaction rollback. |
| slice | A growable view over an array: `[]Product` | array | `nil` is a valid empty slice. `append` can allocate a new backing array. |
| map | Key/value collection: `map[string]any` | object / `Map` | Reading a missing key returns the value type's zero value; the two-value form tells whether it existed. |
| struct tag | Metadata after a field, for example ``json:"email"`` | decorators / serializer config | Libraries use tags for JSON, configuration, and validation at runtime. |
| goroutine/channel | Lightweight concurrent function and typed communication channel | async task / queue | `go fn()` starts concurrently; `chan error` communicates a result. It is not a JavaScript Promise. |

**Rule of thumb while reading:** find the returned `error`, then follow the `if err != nil` branch. In Go that branch is the visible equivalent of exception handling.

---

## 2. What you need installed

- **Go 1.26+** (`go version`; the required version is declared in `go.mod`)
- **Docker Desktop** (for PostgreSQL, Valkey and the full stack)
- **VS Code** with the **REST Client** extension (or **Thunder Client**) to test the API
- `make` (Mac/Linux have it; on Windows use WSL or Git Bash)

### Windows note: `make` is optional for the Docker path

This repository is being run on Windows. Docker Desktop includes `docker compose`, but it does
not include GNU Make. You can either use **WSL/Git Bash** and run the `make` commands shown below,
or run the equivalent PowerShell command directly:

```powershell
# From the repository root; this is the Windows equivalent of `make up`.
docker compose -f deploy/compose/docker-compose.dev.yml up --build -d

# Equivalent of `make logs` and `make down`.
docker compose -f deploy/compose/docker-compose.dev.yml logs -f api worker
docker compose -f deploy/compose/docker-compose.dev.yml down
```

Use `Copy-Item .env.example .env` rather than `cp .env.example .env` in PowerShell. Keep `.env`
local: it contains the JWT secret and bootstrap password, so never commit or paste its values.

---

## 3. How to run it

### First time only

```bash
cp .env.example .env     # settings (the defaults work for local development)
make bootstrap           # installs tools, generates code, creates go.sum
```

`make bootstrap` runs the two code generators:
`oapi-codegen` (API types from `api/v1/openapi.yaml`) and `sqlc` (database code from `db/queries`).
Commit the generated code and `go.sum` afterwards.

### Option A: everything in Docker (easiest)

```bash
make up        # starts PostgreSQL, Valkey, migrations, API, worker, Nginx
make logs      # watch the logs
make down      # stop everything
```

The API is now at **http://localhost:8080**.

#### Confirm that the stack is ready

`up` starts PostgreSQL and Valkey, runs the SQL and River migrations, then starts API, worker, and
Nginx. Docker returning from the command only means containers were started—it does not guarantee
the API is ready. Wait for the following request to return `200` and `{"status":"ready"}`:

```powershell
curl.exe -i http://localhost:8080/readyz
```

If it returns `503` or does not connect, inspect the two application processes:

```powershell
docker compose -f deploy/compose/docker-compose.dev.yml logs --tail=100 api worker
```

When you are done, stop the stack with `make down` (WSL/Git Bash) or the PowerShell `docker compose
... down` command shown above. Add `-v` only when you deliberately want to delete your local
PostgreSQL volume and all local data.

### Option B: Go on your machine, databases in Docker (best for coding)

```bash
docker compose -f deploy/compose/docker-compose.dev.yml up -d postgres valkey
make migrate-up        # create the tables
make river-migrate     # create the job-queue tables
make run-api           # terminal 1: API on http://localhost:8080 (hot restart: Ctrl+C, run again)
make run-worker        # terminal 2: background worker
```

With option B the API is on port **8080** too (set by `OXUS_HTTP_ADDR` in `.env`).

On the very first start the API creates one admin from `OXUS_BOOTSTRAP_ADMIN_EMAIL`
and `OXUS_BOOTSTRAP_ADMIN_PASSWORD` in `.env`
(default `admin@example.com` / `change-me-please-123`). Remove those two lines later.

---

## 4. Test the API

### 4a. REST Client extension (recommended, a ready file is included)

1. In VS Code install **REST Client** (`humao.rest-client`).
2. Open **`docs/api.http`**.
3. Click **Send Request** above `# @name login`. You should see `200 OK` with an `access_token`.
4. Click **Send Request** above any other request. The token and the new product id are filled in automatically.
5. Suggested order: *Create a product → List → Get → Update → Export → Delete*.
6. Try the "Error cases" at the bottom to see how errors look.

### 4b. Thunder Client extension

1. Install **Thunder Client** (`rangav.vscode-thunder-client`) and click its icon.
2. **New Request** → `POST` → `http://localhost:8080/api/v1/admin/auth/login`
   → Body tab → JSON:
   ```json
   { "email": "admin@example.com", "password": "change-me-please-123" }
   ```
   → **Send**. Copy the `access_token` from the response.
3. For every other request open the **Auth** tab → choose **Bearer** → paste the token.
   (Tip: save the token in an *Environment* variable and use `{{token}}`.)
4. Requests to try:

| Method | URL | Body |
|---|---|---|
| POST | `/api/v1/admin/products` | `{"code":"MURABAHA1","name":"Murabaha","currency":"AFN","min_amount":"1000.00","max_amount":"50000.00","annual_rate":"12.50","status":"active"}` |
| GET | `/api/v1/admin/products?page=1&page_size=20` | none |
| GET | `/api/v1/admin/products/{id}` | none (use the `id` from the create response) |
| PUT | `/api/v1/admin/products/{id}` | `{"name":"New name","description":"x","min_amount":"2000.00","max_amount":"60000.00","annual_rate":"11.00","status":"active"}` |
| DELETE | `/api/v1/admin/products/{id}` | none (expect `204`) |
| GET | `/api/v1/admin/exports/products` | none (downloads an `.xlsx` file) |

   Thunder Client can also import `api/v1/openapi.yaml` (Collections → menu → Import) if your version supports OpenAPI import.

### Understanding the responses

| Status | Meaning |
|---|---|
| 200 / 201 / 204 | OK / created / deleted |
| 400 | Invalid data. The `details` list tells you which field |
| 401 | No token, bad token, or wrong password |
| 403 | Your role is not allowed (the `viewer` role is read-only) |
| 404 | Not found |
| 409 | Duplicate product code |
| 429 | Too many requests, or account locked after 5 wrong passwords (15 min) |

Errors always look like:
`{"error":{"code":"validation_failed","message":"validation failed","details":[{"field":"max_amount","message":"must be greater than or equal to min_amount"}]}}`

---

## 5. Run the automated tests

```bash
make test               # fast unit tests, no Docker needed
make test-integration   # real PostgreSQL in Docker (Testcontainers)
make lint               # code quality + security checks
make vuln               # known-vulnerability check of libraries
```

---

## 6. Architecture: follow one request end to end

```text
HTTP client
   │
   ▼
Gin router + middleware
   │  request ID, recovery, logging, metrics, body size, rate limit, JWT
   ▼
HTTP handler
   │  JSON / generated OpenAPI type ↔ domain type
   ▼
Service
   │  permissions, validation, transaction boundary, audit/event decisions
   ▼
Repository
   │  sqlc query → pgx → PostgreSQL
   ├─────────────────────► audit_logs (same transaction)
   └─────────────────────► River job (same transaction) → worker
```

A request has two layers of types:

1. **Generated API types** in `internal/httpapi/v1/gen` exactly match `api/v1/openapi.yaml`.
2. **Domain types** in `internal/auth` and `internal/products` express the application's rules.

Handlers deliberately convert between them. This keeps HTTP conventions (optional JSON fields,
response field names) out of business rules and lets the service be unit-tested without Gin.

### Why product changes use a transaction

Creating, updating, and deleting a product needs three durable effects: the product row, an audit
row, and a River background-job row. `database.DB.WithinTx` begins a PostgreSQL transaction,
places it in the `context.Context`, and commits only if its callback returns `nil`. Any returned
error causes the deferred rollback. Repositories and the job publisher retrieve that transaction
from the context, so all three writes commit together or none do. This is similar to passing a
single transaction object through a TypeScript service call chain, but Go's context carries it
without changing every repository method signature.

### Important directories and non-Go files

| Path | Purpose | When you change it |
|---|---|---|
| `Makefile` | Repeatable developer commands: generation, test, lint, containers, migrations. | Prefer a target over remembering a long command. |
| `go.mod` / `go.sum` | Module identity, direct/transitive versions, and immutable dependency checksums. | Run `go mod tidy` after intentionally adding/removing imports. Commit both files. |
| `api/v1/openapi.yaml` | API contract: paths, request schemas, responses, and which operations require bearer authentication. | Edit this first for API-shape changes, then run `make generate`. |
| `api/v1/oapi-codegen.yaml` | Generation configuration for the OpenAPI server/types. | Rarely; change only when generation output must change. |
| `db/migrations/*.up.sql` | Forward schema changes. | Add a new numbered migration; never edit an already-applied production migration. |
| `db/migrations/*.down.sql` | Reverse schema change for local/dev rollback. | Keep it synchronized with its matching `up` migration. |
| `db/queries/*.sql` | Named SQL statements read by sqlc. | Edit for database behavior, then run `make generate`; do not hand-write the generated equivalents. |
| `db/sqlc.yaml` | sqlc input/output/type-mapping configuration. | Only when adding query packages or changing generated type behavior. |
| `deploy/compose/docker-compose.dev.yml` | Development PostgreSQL, Valkey, migration/API/worker/Nginx containers. | Change local stack topology or environment wiring. |
| `deploy/docker/Dockerfile` | Multi-stage image build for API and worker. | Change runtime/build dependencies. |
| `deploy/nginx/nginx.conf` | Reverse proxy configuration in the Docker stack. | Change proxy, TLS, or public HTTP behavior. |
| `docs/api.http` | Executable REST Client tutorial with variables captured from responses. | Add or update documented API examples. |
| `bin/api.exe` | A locally built Windows executable. | Regenerate through the Makefile/build; it is not hand-maintained source. |

---

## 7. Go source map: every package, file, and key function

> **Generated files:** Never manually edit `internal/httpapi/v1/gen/api.gen.go` or files in
> `internal/platform/database/sqlc/`. Their inputs are the OpenAPI/SQL files above; regenerate
> them with `make generate`.

### Application entry points (`cmd`)

| File | Main functions and important logic |
|---|---|
| `cmd/api/main.go` | `main` calls `run` and prints a startup failure to stderr. `run` is the **composition root**: it loads `Config`, opens PostgreSQL/Valkey, constructs each module, creates the River enqueue-only client, starts `http.Server`, and waits on either a server error or OS cancellation. `signal.NotifyContext` turns Ctrl+C/SIGTERM into a cancelable context. The `go func` runs the blocking server concurrently; its buffered `chan error` reports unexpected failure. `Shutdown` uses a fresh timeout context so active requests can finish. `newLogger` builds structured JSON `slog` output. |
| `cmd/worker/main.go` | `main`/`run` start the separate River worker process. It registers `products.ChangedWorker`, which consumes jobs previously inserted by API transactions. The `worker migrate` mode creates River's own queue tables. A separate process means slow background work cannot block HTTP responses. |

### Error and audit packages

| File | Main functions and important logic |
|---|---|
| `internal/apperr/apperr.go` | Defines typed, safe-to-return application errors: `Validation`, `Unauthorized`, `Forbidden`, `NotFound`, `Conflict`, and `TooManyRequests`. `As` and `Is` use Go's error-unwrapping conventions, so outer layers can decide an HTTP status without exposing database or internal errors. |
| `internal/audit/model.go` | Defines `Entry`, the domain record for who did what to which entity. Its internal `record` representation adds HTTP request metadata before storage. |
| `internal/audit/repository.go` | `PgRepository.Insert` converts an audit entry to sqlc parameters and inserts it. It reads the active transaction from the context through `database.DB.Queries`, so it joins a product-write transaction when one exists. |
| `internal/audit/service.go` | `NewService` wires its repository. `Record` marshals flexible `map[string]any` metadata to JSON and collects request ID/client IP from `reqctx`; it is the only audit API feature services need. |

### Authentication (`internal/auth`)

| File | Main functions and important logic |
|---|---|
| `model.go` | Defines `Role`, `User`, `Principal`, login input/output types. `Role.Valid`, `CanWrite`, and `CanExport` centralize authorization rules instead of scattering role string comparisons. |
| `handler.go` | `Handler.Login` binds JSON, calls the service, returns the access token, and sets `Cache-Control: no-store` so intermediaries do not cache credentials. |
| `service.go` | `Login` normalizes/validates email, checks Valkey failure count, authenticates, resets count on success, issues JWT, and records audit data. `authenticate` always verifies a password hash, even for unknown users, to avoid timing-based email enumeration. `registerFailure` increments lockout attempts but logs audit failures rather than hiding the login result. `EnsureBootstrapAdmin` creates the initial super-admin only if the admin count is zero; a duplicate-key race is safe because another instance won. |
| `password.go` | `HashPassword` generates a cryptographically random salt then produces a PHC-formatted Argon2id hash. `VerifyPassword` parses that format and uses `subtle.ConstantTimeCompare`; unlike `===`, it avoids leaking where two values differ through timing. `sync.OnceValue` builds `dummyHash` once, safely shared by concurrent requests. |
| `token.go` | `NewTokenManager` stores signing settings. `Issue` puts user ID and role into a short-lived HS256 JWT. `Parse` validates algorithm, issuer, expiry, and claims before returning an authenticated `Principal`; it must not trust unverified token fields. |
| `principal.go` | `WithPrincipal`/`PrincipalFrom` put/get the authenticated caller on `context.Context`. The private key type prevents collisions with other packages. `PrincipalFromGin` adapts Gin's request context and produces a typed 401 error if middleware did not authenticate. |
| `repository.go` | `PgRepository` uses generated sqlc queries for admin users. `GetByEmail`, `Count`, and `Create` translate database-specific errors into `apperr` kinds and map generated database rows into `User`. |
| `password_test.go` | Tests hash/verify round trips, salts, and bad hash input. |
| `service_test.go` | Uses fakes that implicitly satisfy the package interfaces to test success, validation, lockout, auditing, and bootstrap behavior without PostgreSQL/Valkey. |
| `token_test.go` | Tests valid-token round trip and token rejection. |

### Products (`internal/products`)

| File | Main functions and important logic |
|---|---|
| `model.go` | Defines product status, `Product`, request inputs, filters, and paged results. Validation tags describe simple field constraints. Money constraints remain in the service because they require comparing multiple values. |
| `handler.go` | `ListProducts`, `CreateProduct`, `GetProduct`, `UpdateProduct`, `DeleteProduct`, and `ExportProducts` are thin HTTP adapters. `toAPI` preserves money as fixed two-decimal **strings**, avoiding JSON floating-point loss. `toCreateInput`/`toUpdateInput` use `decimals.parse` to reject exponent notation and aggregate field errors before decimal conversion. |
| `service.go` | `Create`, `Update`, and `Delete` enforce `Principal` permissions, normalize input, validate, and execute data/audit/job work in `WithinTx`. `List` validates page, size, status, and search limits before querying. `Export` requires export permission, creates XLSX bytes, then requires a successful audit write. `validateMoney` enforces plain decimal format, range, and min/max relationships; `record` requires the audit entry and event enqueue to succeed. |
| `repository.go` | `PgRepository` delegates CRUD to sqlc and maps `pgx`/PostgreSQL conditions to domain errors. `Delete` is a **soft delete**: it sets `deleted_at`, preserving history instead of physically removing a row. `toProduct`/`toProducts` isolate generated SQL row shapes. |
| `export.go` | `buildWorkbook` creates an in-memory Excel workbook; `setRow` writes cells. Amounts are written as strings rather than `float64`, because binary floating-point is unsafe for finance values. |
| `jobs.go` | Defines `ChangedArgs` and `ChangedWorker` for the `products.changed` River job. `RiverPublisher.ProductChanged` requires the current database transaction and inserts the job through it, implementing an outbox-style atomic write. The worker currently logs the event, leaving a safe extension point for notifications/indexing. |
| `repository_integration_test.go` | Runs against real PostgreSQL via Testcontainers, checking CRUD and rollback semantics that a fake cannot prove. |
| `service_test.go` | Unit-tests permission checks, input normalization, validation, audit/event calls, pagination, and workbook creation. |

### HTTP API and middleware

| File | Main functions and important logic |
|---|---|
| `internal/httpapi/v1/router.go` | `NewRouter` configures Gin mode and trusted proxies, disallows unknown JSON fields, applies global middleware, adds health/readiness/metrics endpoints, then registers generated OpenAPI routes at `/api/v1`. `/readyz` has its own short deadline and checks both dependencies. The generated middleware marks operations that declare `bearerAuth`, allowing auth policy to stay in the contract. |
| `internal/httpapi/v1/server.go` | `Server` satisfies the generated OpenAPI interface and delegates each endpoint to its feature handler. `GetAppConfig` returns minimum supported app version and server time. This file is a routing adapter, not a place for business rules. |
| `internal/httpapi/v1/gen/api.gen.go` | Generated request/response structs, interfaces, and Gin bindings. It converts path/query input to typed arguments and attaches security metadata. Regenerate; do not edit. |
| `internal/middleware/auth.go` | `Authenticate` only runs on generated routes marked protected, extracts `Authorization: Bearer <token>`, validates the JWT, then installs the `Principal` in request context. `bearerToken` uses `strings.Cut` and case-insensitive scheme matching. |
| `internal/middleware/requestid.go` | `RequestID` accepts a valid client ID or creates one, returns it in a header, and attaches request ID/client IP to context. `BodyLimit` wraps the request body so oversized JSON fails safely. |
| `internal/middleware/logging.go` | `Logging` emits structured request records without query strings. `Recovery` catches a panic, logs its stack, and returns an internal error rather than crashing the process. |
| `internal/middleware/metrics.go` | `NewMetrics` creates Prometheus counters/histograms. `Middleware` records the normalized Gin route pattern—not raw URLs—to avoid unbounded metric labels. `Handler` exposes Prometheus text at `/metrics`. |
| `internal/middleware/ratelimit.go` | `RateLimit` increments a Valkey fixed-window counter by client IP, skips internal routes, sends `Retry-After` for a 429, and intentionally fails open if Valkey is unavailable so a cache outage does not take down the API. |

### Platform/shared packages

| File | Main functions and important logic |
|---|---|
| `internal/config/config.go` | `Load` merges defaults and `OXUS_*` environment variables through koanf, decodes them into `Config`, then calls `validate`. `IsProduction` controls production behavior. `validate` accumulates errors with `errors.Join`, so startup reports all invalid settings at once. `time.Duration` fields accept values such as `15m`; this is a typed duration, not an unvalidated string. |
| `internal/platform/database/database.go` | `NewPool` parses/configures a pgx pool, registers `NUMERIC` ↔ `decimal.Decimal` conversion, verifies connectivity, and returns it. `Queries` selects the current transaction or shared pool. `WithinTx` creates/commits/rolls back a transaction; `defer` guarantees rollback cleanup even on early return. Nested calls reuse the existing transaction. `IsUniqueViolation` inspects a typed PostgreSQL error code. |
| `internal/platform/database/sqlc/db.go` | Generated sqlc plumbing: `DBTX` is the minimal database interface; `New`, `Queries`, and `WithTx` construct query sets. |
| `internal/platform/database/sqlc/models.go` | Generated Go representations of database tables and query rows. |
| `internal/platform/database/sqlc/auth.sql.go` | Generated implementations of `CountAdminUsers`, `CreateAdminUser`, and `GetAdminUserByEmail`. |
| `internal/platform/database/sqlc/audit.sql.go` | Generated implementation of `InsertAuditLog`. |
| `internal/platform/database/sqlc/products.sql.go` | Generated implementations of product create/read/list/update/soft-delete/export queries. |
| `internal/platform/httpx/errors.go` | `WriteError` turns typed `apperr` values into the standard JSON envelope and hides unexpected internals behind a generic 500. `BindJSON` and `ParamErrorHandler` convert Gin/OpenAPI binding failures to the same response format. Generic `ValueOr[T]` chooses a pointer's value or a default. |
| `internal/platform/reqctx/reqctx.go` | Defines request metadata and `With`/`From` helpers using a private context key, preventing accidental key collisions. |
| `internal/platform/validation/validation.go` | `Struct` validates tagged structs with a shared validator. It translates tag failures into friendly JSON field names and application validation errors, rather than exposing Go field names. |
| `internal/platform/valkey/valkey.go` | `NewClient` connects/pings Valkey. `Limiter.Hit` increments a fixed-window counter and assigns TTL only when first created (`ExpireNX`); `Count` reads it and `Reset` clears it. The same small interface powers global rate limiting and login lockout. |

### Reading order for a beginner

1. Start with `cmd/api/main.go` to see how pieces are created.
2. Read `internal/httpapi/v1/router.go`, then `internal/middleware/auth.go` to see request setup.
3. Follow `internal/products/handler.go` → `service.go` → `repository.go` for the main CRUD feature.
4. Read `internal/platform/database/database.go` to understand the transaction behavior.
5. Read `internal/auth/service.go`, `password.go`, and `token.go` for authentication.
6. Finally inspect generated code only to understand its shape—change the OpenAPI/SQL source files instead.

---

## 8. Endpoints

| Method | Path | Who |
|---|---|---|
| GET | `/api/v1/app/config` | public |
| POST | `/api/v1/admin/auth/login` | public |
| GET | `/api/v1/admin/products` | any admin role |
| POST | `/api/v1/admin/products` | admin, super_admin |
| GET | `/api/v1/admin/products/{id}` | any admin role |
| PUT | `/api/v1/admin/products/{id}` | admin, super_admin |
| DELETE | `/api/v1/admin/products/{id}` | admin, super_admin |
| GET | `/api/v1/admin/exports/products` | admin, super_admin |
| GET | `/healthz`, `/readyz`, `/metrics` | internal use |

---

## 9. Libraries used (from the specification)

Gin (web), oapi-codegen (API contract), pgx + sqlc + golang-migrate (database),
validator (input checks), golang-jwt + Argon2id (login), River (background jobs),
go-redis on Valkey (rate limit), shopspring/decimal (money), excelize (Excel),
slog + koanf + Prometheus (logs, config, metrics), testify + testcontainers (tests).

**Not added yet (nothing needs them):** circuit breaker, Firebase push, S3/SeaweedFS files,
Gotenberg PDFs, Vault, 2FA, Sentry. Add each when its feature is built.

---

## 10. Troubleshooting

| Problem | Fix |
|---|---|
| `cannot find package ... sqlc` or `gen` | Run `make generate` |
| `missing go.sum entry` | Run `go mod tidy` |
| `OXUS_JWT_SECRET must be at least 32 characters` | Check your `.env` file exists and has the secret |
| `connection refused` on port 5432 or 6379 | Start the databases: `docker compose -f deploy/compose/docker-compose.dev.yml up -d postgres valkey` |
| Login returns 429 | Too many wrong passwords. Wait 15 minutes or clear Valkey: `docker compose -f deploy/compose/docker-compose.dev.yml restart valkey` |
| Compile error about a field name like `Id` vs `ID` | Different oapi-codegen version; rename that field in `internal/products/handler.go` |
