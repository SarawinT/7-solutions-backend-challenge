# user-management-api

A user management service in Go. One application core serves both a **REST API (Fiber v3)** and a **gRPC API**, with **JWT authentication** (HS256) on both, backed by **MongoDB**.

The project uses ports and adapters: handlers and services are split per feature, with one file per use case. The core (`app/core`, `app/models`) stays free of Fiber, gRPC and database code.

## Layout

```
.
├── main.go                     # entry point: config, logger, MongoDB, middleware, routers, graceful shutdown
├── config/
│   ├── config.go               # YAML config (Viper) + env overrides + validation
│   ├── config.example.yaml     # template (committed)
│   └── config.yaml             # your local config with secrets (gitignored)
├── logz/                       # logger setup (log/slog, JSON)
├── middleware/                 # Fiber middleware: request ID, request log, recover, JWT, error handler
├── app/
│   ├── adapters/
│   │   ├── handlers/
│   │   │   ├── handler_auth/   # REST /api/v1/auth: register.go, login.go, me.go
│   │   │   └── handler_user/   # REST /api/v1/users: inquiry_user.go, list_user.go, update_user.go, delete_user.go
│   │   ├── grpc/               # gRPC handlers, interceptors (logging, recovery, JWT), error → status
│   │   ├── repository/
│   │   │   └── mongodb/        # MongoDB repositories, one file per collection
│   │   │       ├── user_repository.go  # ports.UserPort
│   │   │       └── mongotest/  # test helper: isolated MongoDB database per test
│   │   └── security/           # JWT (HS256), bcrypt, UUIDv7: implement the security ports
│   ├── constants/              # response codes, context keys, headers
│   ├── core/
│   │   ├── ports/              # interfaces the services depend on (UserPort, TokenPort, ...)
│   │   └── services/
│   │       ├── service_auth/   # AuthServicer: register.go, login.go, verify_token.go
│   │       └── service_user/   # UserServicer: inquiry_user.go, list_user.go, update_user.go, delete_user.go
│   ├── database_init/          # MongoDB client
│   ├── models/
│   │   ├── dto/                # service results (ListUserDto, LoginDto)
│   │   ├── entity/             # UserEntity + rules, business errors
│   │   ├── request/            # request models with Validate() (shared by REST and gRPC)
│   │   └── response/           # {code, message, data} envelope + response models
│   ├── routers/                # wires repository → service → handler, registers routes (REST + gRPC)
│   └── utils/                  # pure helpers (pagination, time)
├── api/proto/                  # gRPC contracts: auth/v1, user/v1
├── gen/go/                     # generated protobuf + gRPC code (make proto)
├── docs/                       # JWT guide, API examples, design decisions, architecture
└── docker-compose.yml          # local MongoDB
```

**Dependency rule:** `main → routers → adapters → core/models`. Nothing in `app/core` or `app/models` imports Fiber, gRPC, MongoDB, a JWT library, or any adapter. See [docs/architecture.md](docs/architecture.md).

## Documentation

| Doc | What's in it |
|---|---|
| [docs/jwt.md](docs/jwt.md) | How to generate, use, decode and troubleshoot JWT tokens |
| [docs/api-examples.md](docs/api-examples.md) | Real request/response samples for every REST endpoint and gRPC RPC, including errors |
| [docs/design-decisions.md](docs/design-decisions.md) | Assumptions, design decisions and their trade-offs, known limitations |
| [docs/architecture.md](docs/architecture.md) | Layers, dependency rules, validation and error mapping, how to extend |

## Setup

### Prerequisites

| Tool | Version | Used for |
|---|---|---|
| Go | 1.26+ | building and running |
| Docker + Compose | any recent | local MongoDB (`make up`) |
| `make`, `openssl` | – | commands below, generating the JWT secret |
| `curl`, `jq` | – | REST examples |
| [`grpcurl`](https://github.com/fullstorydev/grpcurl) | – | gRPC examples (optional) |

Already have MongoDB? Skip Docker and set `mongo.uri` (or `MONGO_URI`) to point at it.

### 1. Configure

```bash
cp config/config.example.yaml config/config.yaml   # gitignored
openssl rand -base64 48                            # paste the output into jwt.secret
```

`jwt.secret` is **required**. The server won't start without one or with one shorter than 32 bytes. Every other key has a default; see [Configuration](#configuration).

### 2. Start MongoDB

```bash
make up        # docker compose: mongo:8.2 on localhost:27017, waits until healthy
```

> MongoDB 8.0 refuses to start on Linux kernel 6.19+ ([SERVER-121912](https://jira.mongodb.org/browse/SERVER-121912)), so compose pins `mongo:8.2`.

### 3. Run

```bash
make run       # REST on :8080, gRPC on :50051
```

Or build a binary:

```bash
make build && ./bin/server -config config/config.yaml
```

On startup the server connects to MongoDB, creates its indexes, and logs `HTTP server listening` and `gRPC server listening`. Stop it with Ctrl-C: both servers drain in-flight requests (up to 10s) before exiting.

### 4. Try it

```bash
curl localhost:8080/health
# {"code":"UM00000","message":"success"}

curl --json '{"name":"Alice","email":"alice@example.com","password":"secret123"}' localhost:8080/api/v1/auth/register
TOKEN=$(curl -s --json '{"email":"alice@example.com","password":"secret123"}' localhost:8080/api/v1/auth/login | jq -r .data.accessToken)
curl localhost:8080/api/v1/auth/me -H "Authorization: Bearer $TOKEN"
```

More in [REST API](#rest-api), [gRPC API](#grpc-api) and [docs/api-examples.md](docs/api-examples.md).

### 5. Test

```bash
make test-integration # full test suite against MongoDB (needs `make up`)
make test             # only tests that need no database (MongoDB-backed ones are skipped)
make cover            # full suite with coverage, prints per-function report (needs `make up`)
make cover-html       # open that coverage report in a browser
```

The service tests, the end-to-end REST and gRPC tests (`app/routers`) and the repository tests run against a real MongoDB. `mongotest` gives each test its own throwaway database, which is dropped afterwards. They run when `MONGO_TEST_URI` is set (`make test-integration` sets it). Otherwise they're skipped, so plain `make test` only covers config, middleware, JWT, entity, request validation and pagination.

### Other commands

```bash
make down              # stop MongoDB
make lint              # go vet
make tools && make proto  # regenerate gRPC code after editing api/proto
```

## Configuration

The server reads `config/config.yaml`. Use `-config` to point elsewhere:

```bash
go run . -config /etc/user-management-api/config.yaml
```

| Key                      | Default                     | Description                                     |
|--------------------------|-----------------------------|-------------------------------------------------|
| `jwt.secret`             | **required**                | HMAC-SHA256 signing key, at least 32 bytes      |
| `jwt.ttl`                | `1h`                        | Access token lifetime (Go duration, e.g. `30m`) |
| `jwt.issuer`             | `user-management-api`       | `iss` claim, checked on every token             |
| `mongo.uri`              | `mongodb://localhost:27017` | MongoDB connection string                       |
| `mongo.database`         | `user_management`           | MongoDB database name                           |
| `server.http_port`       | `8080`                      | REST (Fiber) listen port                        |
| `server.grpc_port`       | `50051`                     | gRPC listen port                                |
| `server.grpc_reflection` | `true`                      | Enable gRPC server reflection                   |
| `log.level`              | `info`                      | `debug`, `info`, `warn`, `error`                |

Keys missing from the file use the defaults above. The config is checked at startup, and every problem is reported at once.

**Environment overrides:** an environment variable overrides the matching key: upper-case it and replace `.` with `_`. This is the recommended way to supply secrets in containers and CI without writing them to a file:

```bash
JWT_SECRET="$(openssl rand -base64 48)" MONGO_URI=mongodb://db:27017 go run .
```

## Logging

Logs are JSON on stdout (`log/slog`, set up in `logz`). Every request produces one line with its method, path and execution time:

```json
{"level":"INFO","msg":"http request","service":"user-management-api","method":"POST","path":"/api/v1/auth/register","status":201,"duration_ms":73.734,"request_id":"3lkQaT9U..."}
{"level":"WARN","msg":"grpc request","service":"user-management-api","method":"/user.v1.UserService/GetUser","code":"NotFound","duration_ms":0.412}
```

- **REST:** `middleware.AttachRequestLogger`. **gRPC:** `LoggingInterceptor` in `app/adapters/grpc/interceptor.go`.
- **Level** follows the outcome: `INFO` for success, `WARN` for client errors (4xx, or gRPC codes like `NotFound` and `Unauthenticated`), `ERROR` for server errors (5xx, `Internal`).
- **Both are the outermost layer**, so requests that panic or are rejected by auth are still logged.
- **`duration_ms`** is in milliseconds with microsecond precision.

## Authentication

1. **Register** with `POST /api/v1/auth/register` (gRPC: `auth.v1.AuthService/Register`).
2. **Log in** with `POST /api/v1/auth/login` (gRPC: `auth.v1.AuthService/Login`). The response contains `accessToken`.
3. **Send the token** on every protected request:
   - REST: header `Authorization: Bearer <token>`
   - gRPC: metadata `authorization: Bearer <token>`

Tokens are HS256 JWTs signed with `jwt.secret` and valid for `jwt.ttl` (default 1h). Public: register, login, `/health`, gRPC health and reflection. Everything else needs a token. There's no refresh or revocation: when a token expires, log in again.

**See [docs/jwt.md](docs/jwt.md)** for the full guide: generating the secret, the claims inside a token, how verification works, error responses and the design's limits.

## REST API

Every response uses the same envelope. `data` is omitted when there's nothing to return:

```json
{"code": "UM00000", "message": "success", "data": {"id": "...", "name": "Alice", "email": "alice@example.com", "createdAt": "...", "updatedAt": "..."}}
{"code": "UM42200", "message": "name is required, password is required"}
```

Full request/response samples, including every error case, are in [docs/api-examples.md](docs/api-examples.md).

Setup: register, log in, and keep the token and your user id for the examples below (`jq` required):

```bash
curl --json '{"name":"Alice","email":"alice@example.com","password":"secret123"}' localhost:8080/api/v1/auth/register
TOKEN=$(curl -s --json '{"email":"alice@example.com","password":"secret123"}' localhost:8080/api/v1/auth/login | jq -r .data.accessToken)
ID=$(curl -s localhost:8080/api/v1/auth/me -H "Authorization: Bearer $TOKEN" | jq -r .data.id)
```

| Endpoint | Description | Auth | Success | Example |
|---|---|---|---|---|
| `POST /api/v1/auth/register` | Create a new account (public sign-up). The only way to create users | – | 201 | `curl --json '{"name":"Alice","email":"alice@example.com","password":"secret123"}' localhost:8080/api/v1/auth/register` |
| `POST /api/v1/auth/login` | Check email + password and return a JWT access token | – | 200 | `curl --json '{"email":"alice@example.com","password":"secret123"}' localhost:8080/api/v1/auth/login` |
| `GET /api/v1/auth/me` | Get the user the current token belongs to | JWT | 200 | `curl localhost:8080/api/v1/auth/me -H "Authorization: Bearer $TOKEN"` |
| `GET /api/v1/users` | List users, paginated (`page`, `pageSize`) | JWT | 200 | `curl 'localhost:8080/api/v1/users?page=0&pageSize=10' -H "Authorization: Bearer $TOKEN"` |
| `GET /api/v1/users/:id` | Get one user by id | JWT | 200 | `curl localhost:8080/api/v1/users/$ID -H "Authorization: Bearer $TOKEN"` |
| `PATCH /api/v1/users/:id` | Update a user's `name` and/or `email` (partial) | JWT | 200 | `curl -X PATCH --json '{"name":"Alicia"}' localhost:8080/api/v1/users/$ID -H "Authorization: Bearer $TOKEN"` |
| `DELETE /api/v1/users/:id` | Delete a user | JWT | 200 | `curl -X DELETE localhost:8080/api/v1/users/$ID -H "Authorization: Bearer $TOKEN"` |
| `GET /health` | Health check: 200 if MongoDB answers a ping, 503 if not | – | 200 | `curl localhost:8080/health` |

Notes:
- **Register** needs `name`, `email` and `password`. All missing fields are reported at once (`UM42200`). The password must be at least 8 characters.
- **Login** returns `accessToken`, `tokenType`, `expiresAt` and `user` in `data`.
- **`GET /users`** takes `page` (0-based) and `pageSize` (max 100, default 20). `data` is `{users, total}`.
- **`PATCH`** is a partial update: send `name` and/or `email`. Omitted or `null` fields are unchanged, an empty body `{}` is rejected (`UM42200`), and the password can't be changed here.
- **`DELETE`** returns a success envelope with no `data`. The email can be registered again. Tokens already issued to that user stay valid until they expire.
- **Authorization:** any authenticated user can update or delete *any* user. Add an ownership or role check before exposing this publicly (see [design decisions](docs/design-decisions.md#known-limitations)).

### Response codes

| Code | HTTP | Meaning |
|---|---|---|
| `UM00000` | 200 / 201 | Success |
| `UM40000` | 400 | Malformed JSON body |
| `UM40100` | 401 | Missing, malformed, invalid or expired token (with a `WWW-Authenticate: Bearer` header) |
| `UM40101` | 401 | Invalid email or password |
| `UM40400` | 404 | Unknown route |
| `UM40401` | 404 | User not found |
| `UM40500` | 405 | Method not allowed |
| `UM40900` | 409 | Email already exists |
| `UM42200` | 422 | Validation failed: missing or invalid field (the message says which) |
| `UM50000` | 500 | Internal error (details are logged, not returned) |
| `UM50300` | 503 | Database not connected (`/health`) |

## gRPC API

Setup: same idea as REST. Register, log in, and keep the token and your user id:

```bash
ID=$(grpcurl -plaintext -d '{"name":"Alice","email":"alice@example.com","password":"secret123"}' localhost:50051 auth.v1.AuthService/Register | jq -r .user.id)
TOKEN=$(grpcurl -plaintext -d '{"email":"alice@example.com","password":"secret123"}' localhost:50051 auth.v1.AuthService/Login | jq -r .accessToken)
```

| RPC | Description | Auth | Example |
|---|---|---|---|
| `auth.v1.AuthService/Register` | Create a new account (public sign-up). The only way to create users | – | `grpcurl -plaintext -d '{"name":"Alice","email":"alice@example.com","password":"secret123"}' localhost:50051 auth.v1.AuthService/Register` |
| `auth.v1.AuthService/Login` | Check email + password and return a JWT access token | – | `grpcurl -plaintext -d '{"email":"alice@example.com","password":"secret123"}' localhost:50051 auth.v1.AuthService/Login` |
| `user.v1.UserService/GetUser` | Get one user by id | JWT | `grpcurl -plaintext -H "authorization: Bearer $TOKEN" -d '{"id":"'$ID'"}' localhost:50051 user.v1.UserService/GetUser` |
| `user.v1.UserService/ListUsers` | List users, paginated (`page`, `page_size`) | JWT | `grpcurl -plaintext -H "authorization: Bearer $TOKEN" -d '{"page":0,"page_size":10}' localhost:50051 user.v1.UserService/ListUsers` |
| `user.v1.UserService/UpdateUser` | Update a user's `name` and/or `email` (partial) | JWT | `grpcurl -plaintext -H "authorization: Bearer $TOKEN" -d '{"id":"'$ID'","name":"Alicia"}' localhost:50051 user.v1.UserService/UpdateUser` |
| `user.v1.UserService/DeleteUser` | Delete a user | JWT | `grpcurl -plaintext -H "authorization: Bearer $TOKEN" -d '{"id":"'$ID'"}' localhost:50051 user.v1.UserService/DeleteUser` |
| `grpc.health.v1.Health/Check` | Health check: is the server serving | – | `grpcurl -plaintext localhost:50051 grpc.health.v1.Health/Check` |

The RPCs run the same services and validation as REST, including the partial update (`UpdateUser` fields are proto3 `optional`). `DeleteUser` returns `{}`.
gRPC uses its own status codes instead of the REST envelope: `Unauthenticated` (missing/invalid token, bad credentials), `NotFound`, `AlreadyExists`, `InvalidArgument` (missing or invalid fields), `Internal`.
