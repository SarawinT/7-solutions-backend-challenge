# Assumptions and design decisions

This file records what the service assumes about its use and why it's built the way it is. For the package layout and dependency rules, see [architecture.md](architecture.md).

## Assumptions

These are product decisions made without an explicit spec. Each one is cheap to change if it's wrong.

| # | Assumption | Consequence in the code |
|---|---|---|
| A1 | **Sign-up is public, and it is the only way to create a user.** No admin creates accounts for others. | No `POST /api/v1/users` or `CreateUser` RPC. `register` is the single entry point. |
| A2 | **Every authenticated user is trusted equally.** There are no roles or owners. | Any valid token can list, read, update or delete *any* user. This is the main thing to add before public use (see [Known limitations](#known-limitations)). |
| A3 | **Email is the login identifier and is unique, ignoring case.** | Emails are trimmed and lower-cased before they're stored or compared. A unique MongoDB index on `email` enforces it, so `Alice@Example.com` and `alice@example.com` are the same account. |
| A4 | **Users can't change their password through the API.** | `PATCH /users/:id` accepts only `name` and `email`. A password change or reset flow is out of scope. |
| A5 | **Delete is permanent.** | Hard delete, no soft-delete flag. The email can be registered again straight away. |
| A6 | **Clients can tolerate re-logging in when a token expires.** | Short-lived access tokens only: no refresh tokens and no server-side sessions. |
| A7 | **Validation rules are minimal.** | Name: 1–100 characters. Email: a basic format check. Password: at least 8 characters, with no complexity rules. |
| A8 | **The user list is small enough for offset paging.** | `page`/`pageSize` (0-based, default 20, max 100) plus `total`. Results are sorted by `(created_at, _id)`, so the order is stable. |
| A9 | **One MongoDB instance (or replica set) is enough.** No cross-collection transactions are needed. | Every operation is a single-document write. |
| A10 | **It runs behind TLS termination** (load balancer or ingress). | Both servers listen in plaintext. gRPC uses `insecure` credentials in tests and examples. |

## Design decisions

### Architecture

**D1. Ports and adapters (hexagonal).** Business rules live in `app/core` and `app/models` and import no framework, driver or crypto library. They depend only on interfaces in `app/core/ports`. *Why:* the same services back two transports (REST and gRPC), the database can be swapped without touching business logic, and services can be tested without HTTP. The build checks this: `go list -deps ./app/core/...` must not include Fiber, gRPC, Mongo, JWT or bcrypt.

**D2. One binary serves REST and gRPC.** `main.go` starts Fiber on `:8080` and gRPC on `:50051` from the same services, and shuts both down gracefully. *Why:* one deployable, one config, and no drift between the two APIs. *Trade-off:* the two can't be scaled independently. Splitting them later only means a second `main`.

**D3. Validation lives in the core, not the transport.** `request.*.Validate()` checks that required fields are present and reports all missing fields at once. The entity checks values (length, format). *Why:* both REST and gRPC get identical rules for free, and no validation library ends up in the core.

**D4. Services return entity errors; each transport maps them.** `ErrUserNotFound` becomes 404/`UM40401` in REST (`response.FromError`) and `NotFound` in gRPC (`mapper.go`). The repository turns driver errors into entity errors too, for example a duplicate key becomes `ErrEmailAlreadyExists`. *Why:* the core never sees HTTP status codes or Mongo error types.

**D5. Repository constructors return the port** (`NewUserRepository(db) ports.UserPort`). MongoDB-specific setup such as index creation is a separate function, `EnsureUserIndexes`, called at startup. *Why:* callers can only use the contract, so an alternative store just has to implement `UserPort` and pass the same contract tests.

### API shape

**D6. REST uses a `{code, message, data}` envelope with stable codes** (`UM` + HTTP status + sequence). *Why:* clients can branch on `code`, which distinguishes cases that share a status code (`UM40100` bad token vs. `UM40101` bad credentials, `UM40400` unknown route vs. `UM40401` unknown user), and every error looks the same. Messages are English only; there's no i18n.

**D7. 400 means unparseable; 422 means parseable but invalid.** Malformed JSON gets `400 UM40000`. Missing or invalid fields get `422 UM42200`.

**D8. `PATCH` is a true partial update.** Fields are pointers (proto3 `optional` in gRPC), so "not sent" and "sent empty" are different things. An empty body is rejected rather than treated as a no-op, because it's almost always a client bug.

**D9. `DELETE` returns `200` with the envelope**, not `204`. *Why:* every response then has the same envelope, and clients don't need a special case for an empty body.

**D10. JSON is camelCase** (`accessToken`, `createdAt`, `pageSize`), which matches proto3's JSON mapping, so REST and gRPC (via grpcurl or a gateway) look alike.

**D11. IDs are UUIDv7 strings generated by the application**, not Mongo `ObjectId`s. *Why:* they're time-ordered (index-friendly), don't depend on the database, and are the same type in every transport.

**D12. Timestamps are UTC and truncated to milliseconds**, the precision MongoDB stores. Otherwise the response to a write would show nanoseconds that a later read can't reproduce.

### Security

**D13. JWT HS256 with a pinned algorithm.** Only `HS256` is accepted, which blocks `alg: none` and RS/HS confusion. The issuer and expiry are required, and the secret must be at least 32 bytes (RFC 7518 §3.2). 30s of clock skew is allowed. *Why HS256:* a single service both issues and verifies tokens, so a symmetric key is simplest. See [jwt.md](jwt.md#7-limits-of-this-design) for when to switch.

**D14. Stateless tokens with no revocation.** *Trade-off, accepted:* no session store and no DB hit per request, in exchange for tokens staying valid until `exp` after logout or deletion. Mitigated by a short `jwt.ttl` (default 1h).

**D15. Passwords are hashed with bcrypt** at the default cost (10). They're never logged or returned, and the response models have no password field at all.

**D16. Login doesn't reveal which emails are registered.** An unknown email and a wrong password return the same `401 UM40101`, *and take the same time*: when the email doesn't exist, the service still compares the password against a dummy bcrypt hash. Without that, an unknown email would return tens of milliseconds faster.

**D17. gRPC is deny-by-default.** The auth interceptor lets through only an explicit allowlist (`Register`, `Login`, health, reflection). A new RPC is protected even if nobody remembers to add auth. In REST, protection is set per route group (`/api/v1/users` group, `/me`).

**D18. Internal errors are logged, not returned.** A 500 returns only `internal error`, and the cause goes to the log with the method and path. Panics are recovered (Fiber `Recover`, gRPC `RecoveryInterceptor`), so they never crash the process.

### Operations

**D19. Configuration is YAML (Viper) with environment overrides.** `config/config.yaml` is gitignored and `config.example.yaml` is committed. Any key can be overridden as an env var (`jwt.secret` → `JWT_SECRET`), so secrets never have to be written to disk in containers. Config errors are reported together at startup, and the server refuses to start with an invalid config.

**D20. Structured JSON logs with `log/slog`** (stdlib, no dependency). There's one line per request with method, path, status and `duration_ms`. Request logging is the *outermost* middleware/interceptor, so it also records requests that panic or fail auth. The log level follows the outcome: info for 2xx, warn for 4xx, error for 5xx.

**D21. Tests run against a real MongoDB, not mocks.** Repository contract tests, service tests and end-to-end REST/gRPC tests use `mongotest`, which gives each test a throwaway database. *Why:* the behaviour that matters most (the unique index, duplicate-key mapping, sort order, millisecond timestamps) only exists in the real database. The tests skip when `MONGO_TEST_URI` is unset, so `make test` still works offline.

**D22. MongoDB is pinned to `mongo:8.2`** in docker-compose, because 8.0 and `latest` refuse to start on Linux kernel ≥ 6.19 (SERVER-121912).

## Known limitations

These are deliberately out of scope or not yet done, roughly in priority order:

1. **No authorization.** Any logged-in user can modify or delete any other user (A2). Next step: allow it only when `claims.UserID == id` or the caller has an admin role.
2. **No token revocation or refresh** (D14). Logout is client-side only.
3. **Passwords longer than 72 bytes return 500.** bcrypt rejects them. They should be a 422 with a max-length rule.
4. **Lengths count bytes, not characters.** A 100-character name in Thai or emoji can exceed the 100-byte limit. Counting runes would fix it.
5. **No rate limiting** on login or register, so brute force and enumeration are limited only by bcrypt's cost.
6. **Offset paging** gets slow on very large collections. Switch to cursor paging on `(created_at, _id)`, which is already indexed, if it's ever needed.
7. **No TLS in-process** (A10).
