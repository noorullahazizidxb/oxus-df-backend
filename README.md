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

---

## 2. What you need installed

- **Go 1.25+** (`go version`)
- **Docker Desktop** (for PostgreSQL, Valkey and the full stack)
- **VS Code** with the **REST Client** extension (or **Thunder Client**) to test the API
- `make` (Mac/Linux have it; on Windows use WSL or Git Bash)

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

## 6. Where things are

```
cmd/api, cmd/worker     start-up code (API server and job worker)
api/v1/openapi.yaml     the API contract (source of truth)
db/migrations           SQL that creates the tables
db/queries              SQL that Go code is generated from
docs/api.http           requests for REST Client
internal/auth           login, passwords (Argon2id), tokens (JWT), lockout
internal/products       the CRUD feature (handler -> service -> repository)
internal/audit          "who did what" log
internal/middleware     logging, rate limit, request id, token check, metrics
internal/platform       database, Valkey, validation, shared HTTP helpers
deploy/                 Dockerfile, Docker Compose, Nginx
```

How a request flows: **handler** (reads request) → **service** (rules and permissions)
→ **repository** (SQL). A change to a product, its audit entry and its background job
are saved in **one database transaction**, so they succeed or fail together.

---

## 7. Endpoints

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

## 8. Libraries used (from the specification)

Gin (web), oapi-codegen (API contract), pgx + sqlc + golang-migrate (database),
validator (input checks), golang-jwt + Argon2id (login), River (background jobs),
go-redis on Valkey (rate limit), shopspring/decimal (money), excelize (Excel),
slog + koanf + Prometheus (logs, config, metrics), testify + testcontainers (tests).

**Not added yet (nothing needs them):** circuit breaker, Firebase push, S3/SeaweedFS files,
Gotenberg PDFs, Vault, 2FA, Sentry. Add each when its feature is built.

---

## 9. Troubleshooting

| Problem | Fix |
|---|---|
| `cannot find package ... sqlc` or `gen` | Run `make generate` |
| `missing go.sum entry` | Run `go mod tidy` |
| `OXUS_JWT_SECRET must be at least 32 characters` | Check your `.env` file exists and has the secret |
| `connection refused` on port 5432 or 6379 | Start the databases: `docker compose -f deploy/compose/docker-compose.dev.yml up -d postgres valkey` |
| Login returns 429 | Too many wrong passwords. Wait 15 minutes or clear Valkey: `docker compose -f deploy/compose/docker-compose.dev.yml restart valkey` |
| Compile error about a field name like `Id` vs `ID` | Different oapi-codegen version; rename that field in `internal/products/handler.go` |
