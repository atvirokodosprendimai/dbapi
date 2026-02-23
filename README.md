# dbapi

Hexagonal SQLite-to-JSON API for a flexible KV store and collection-based business records.

## Why this shape

- Domain-first core (`internal/core`) with no HTTP or SQL dependencies.
- Adapters wrap ports:
  - `internal/adapters/sqlite`: persistence adapter.
  - `internal/adapters/httpapi`: delivery adapter.
- Persistence uses `gorm.io` on top of `modernc` SQLite.
- Schema is managed by `goose` SQL migrations.
- Protected routes use API key auth with tenant scoping.
- This makes it easy to add new API endpoints (or a new transport like NATS) without changing core business rules.

## API

### Public

- `GET /healthz`
- `GET /openapi.json`

### Authenticated (X-API-Key or Authorization: Bearer)

- `PUT /v1/kv/{key}`
  - Body: `{ "category": "users", "value": { ...any JSON... } }`
  - Upserts by key.
- `GET /v1/kv/{key}`
  - Direct key lookup.
- `DELETE /v1/kv/{key}`
  - Deletes by key.
- `GET /v1/kv?category=users&prefix=user:&after=user:2&limit=100`
  - Prefix scan with optional category filter and cursor pagination.

#### Collection records (tenant isolated)

- `PUT /v1/collections/{collection}/records/{id}`
  - Body: any JSON object.
- `GET /v1/collections/{collection}/records/{id}`
- `DELETE /v1/collections/{collection}/records/{id}`
- `GET /v1/collections/{collection}/records?prefix=...&after=...&limit=100`
- `POST /v1/collections/{collection}/records:bulk-upsert`
  - Body: `{ "items": [{ "id": "c1", "data": {...} }] }`
- `POST /v1/collections/{collection}/records:bulk-delete`
  - Body: `{ "ids": ["c1", "c2"] }`

`Idempotency-Key` header is supported on bulk endpoints.

## Run

```bash
go run ./cmd/app --addr :8080 --db-path ./dbapi.sqlite --bootstrap-api-key "dev-key" --bootstrap-tenant "tenant-dev"
```

You can also set:

- `DBAPI_BOOTSTRAP_API_KEY`
- `DBAPI_BOOTSTRAP_TENANT`
- `DBAPI_BOOTSTRAP_KEY_NAME`

## Design notes

- Storage table uses JSON validity check (`json_valid`).
- Prefix scan is lexicographic and index-friendly (`key >= prefix` and `key < prefix+"\uffff"`).
- Limit is clamped in use case (`1..1000`) to keep scans predictable.
- Migrations live in `migrations/files` and runner code in `migrations/mig.go`.
- API keys are stored as SHA-256 token hash (not raw token).
- Audit logs are stored in `audit_logs` for record writes/deletes.

## Extend with new endpoints

1. Add a new use case method in `internal/core/usecase`.
2. Add needed repository method in `internal/core/ports` and sqlite implementation.
3. Add new route + handler in `internal/adapters/httpapi`.

Core stays stable while adapters evolve.
