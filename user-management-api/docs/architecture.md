# Architecture

The project uses **ports and adapters**. Business logic lives in `app/core` (services and ports) and `app/models` (entities, request, response and DTO models). It knows nothing about Fiber, gRPC, MongoDB or crypto libraries. Everything that touches the outside world is an adapter in `app/adapters`, wired together in `app/routers` and `main.go`.

```
 main.go ── config/, logz/, middleware/, app/database_init
    │
    ▼
 app/routers  (per feature: repository → service → handler, then routes)
    │
    ├─▶ app/adapters/handlers/handler_auth, handler_user   (REST, Fiber)
    ├─▶ app/adapters/grpc                                   (gRPC)
    │           │ call
    │           ▼
    │   app/core/services/service_auth  (AuthServicer)
    │   app/core/services/service_user  (UserServicer)
    │           │ depend on
    │           ▼
    │   app/core/ports  (UserPort, TokenPort, PasswordHasherPort, IDGeneratorPort)
    │           ▲ implemented by
    ├─▶ app/adapters/repository/mongodb           (MongoDB)
    └─▶ app/adapters/security                     (JWT, bcrypt, UUIDv7)
```

## Feature layout

Each feature has one handler package and one service package, with one file per use case.

| Use case | REST handler | Service | Route |
|---|---|---|---|
| Register | `handler_auth/register.go` | `service_auth/register.go` | `POST /api/v1/auth/register` |
| Login | `handler_auth/login.go` | `service_auth/login.go` | `POST /api/v1/auth/login` |
| Me | `handler_auth/me.go` | `service_user/inquiry_user.go` | `GET /api/v1/auth/me` |
| Inquiry user | `handler_user/inquiry_user.go` | `service_user/inquiry_user.go` | `GET /api/v1/users/:id` |
| List users | `handler_user/list_user.go` | `service_user/list_user.go` | `GET /api/v1/users` |
| Update user | `handler_user/update_user.go` | `service_user/update_user.go` | `PATCH /api/v1/users/:id` |
| Delete user | `handler_user/delete_user.go` | `service_user/delete_user.go` | `DELETE /api/v1/users/:id` |

The service interfaces (`AuthServicer`, `UserServicer`) are defined in their service packages, and handlers depend on those interfaces, not on the concrete structs. The gRPC handlers in `app/adapters/grpc` call the same interfaces.

Services take `context.Context` and request models, and return entities/DTOs and errors. They never see Fiber or HTTP types, which lets the gRPC API reuse them. The handlers build the response envelope.

## Layers

| Package | Role | May import |
|---|---|---|
| `app/models/entity` | `UserEntity` + its rules (name, email, password), business errors, `ValidationError` | stdlib only |
| `app/models/request` | Request models with `Validate()` (required fields) | `entity` |
| `app/models/dto` | Service results (`ListUserDto`, `LoginDto`) | `entity` |
| `app/models/response` | REST envelope, response models, error → (HTTP status, code) mapping | `entity`, `dto`, `constants`, Fiber |
| `app/core/ports` | Interfaces the services need | `entity` |
| `app/core/services/*` | Use cases | `ports`, `models` (not `response`), `utils` |
| `app/adapters/*` | Handlers, gRPC, repository, security | anything above + frameworks/drivers |
| `app/routers`, `main.go` | Wiring | everything |

To verify the core stays clean (no Fiber, gRPC, MongoDB, JWT or bcrypt, even indirectly):

```bash
go list -deps ./app/core/... | grep -E "fiber|google.golang.org/grpc|mongo-driver|golang-jwt|bcrypt" || echo clean
```

## Validation

Validation happens in two layers. Both run inside the services, so REST and gRPC behave the same:

1. **`request.*.Validate()`** checks that required fields are present, e.g. `name is required, password is required`. It reports all missing fields at once.
2. **The entity** checks values: name 1–100 characters, email format (lower-cased and trimmed), password at least 8 characters.

## Error handling

Services return entity errors unchanged. Each transport maps them:

| Error | REST (HTTP / code) | gRPC |
|---|---|---|
| `*ValidationError`, `ErrInvalidName/Email/Password` | 422 / `UM42200` | `InvalidArgument` |
| `ErrUserNotFound` | 404 / `UM40401` | `NotFound` |
| `ErrEmailAlreadyExists` | 409 / `UM40900` | `AlreadyExists` |
| `ErrInvalidCredentials` | 401 / `UM40101` | `Unauthenticated` |
| `ErrInvalidToken`, missing token | 401 / `UM40100` | `Unauthenticated` |
| anything else | 500 / `UM50000` (generic message, cause logged) | `Internal` |

REST mapping lives in `response.FromError`, rendered by `middleware.ErrorHandler`. gRPC mapping lives in `app/adapters/grpc/mapper.go`.

The repository must return these entity errors, not driver errors, so services never depend on storage details. For example, it turns a duplicate-key error on the unique `email` index into `ErrEmailAlreadyExists`.

## Repository

`app/adapters/repository/mongodb` holds the MongoDB repositories, one file per collection. Today that is `user_repository.go` (`ports.UserPort`). Constructors return the port (`NewUserRepository(db) ports.UserPort`). MongoDB-only setup stays outside the port: `EnsureUserIndexes(ctx, db)` runs at startup in `main.go` and creates a unique index on `email`, and `(created_at, _id)` for paging. Storage uses its own `userDocument` type, so bson tags stay out of the entity. `user_repository_test.go` covers the full `UserPort` contract against a real MongoDB.

`mongodb/mongotest` is a test helper, not an adapter. It gives each test an isolated, auto-dropped database, and skips the test when `MONGO_TEST_URI` is unset.

Timestamps are truncated to milliseconds (`utils.NowUTC`), the precision MongoDB stores, so responses match what is read back later.

## Extending

- **New use case:** add `<use_case>.go` to the service package (and its interface), a matching handler file, and a route in the feature's router.
- **New feature:** `app/core/services/service_<name>`, `app/adapters/handlers/handler_<name>`, `app/routers/<name>_router.go`, and a call in `main.go`.
- **New collection:** add `<name>_repository.go` to `repository/mongodb`, implementing a new port in `app/core/ports`.
- **Postgres:** `app/adapters/repository/postgres/user_repository.go` implementing `ports.UserPort`, and make the same `UserPort` contract tests pass against it.
- **Redis cache:** `app/adapters/repository/repository_cache` with a `ports.CachePort`, used by the services that need it.
