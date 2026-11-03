# API examples

These are real requests and the responses the server returned (pretty-printed). IDs, timestamps and tokens will differ on your machine.

All REST examples assume the server is on `localhost:8080` and these shell variables are set (see [jwt.md](jwt.md)):

```bash
TOKEN=$(curl -s --json '{"email":"alice@example.com","password":"secret123"}' localhost:8080/api/v1/auth/login | jq -r .data.accessToken)
ID=$(curl -s localhost:8080/api/v1/auth/me -H "Authorization: Bearer $TOKEN" | jq -r .data.id)
```

- [REST](#rest): [register](#register) · [login](#login) · [me](#me) · [list users](#list-users) · [get user](#get-user) · [update user](#update-user) · [delete user](#delete-user) · [health](#health) · [routing errors](#routing-errors)
- [gRPC](#grpc)

## REST

Every REST response, success or error, has the same envelope:

```json
{ "code": "UM00000", "message": "success", "data": { } }
```

`code` is stable and safe to branch on. `message` is for humans. `data` is omitted when there's nothing to return. The full list of codes is in the [README](../README.md#response-codes).

### Register

`POST /api/v1/auth/register` (public)

```bash
curl -i --json '{"name":"Alice","email":"alice@example.com","password":"secret123"}' \
  localhost:8080/api/v1/auth/register
```

```http
HTTP/1.1 201 Created
```
```json
{
  "code": "UM00000",
  "message": "success",
  "data": {
    "id": "01a1a042-dfdc-7764-b471-de2d8eb15783",
    "name": "Alice",
    "email": "alice@example.com",
    "createdAt": "2026-11-03T07:14:59.676Z",
    "updatedAt": "2026-11-03T07:14:59.676Z"
  }
}
```

The password hash is never returned.

**Email already registered.** Emails are compared case-insensitively, so `Alice@Example.com` counts as a duplicate:

```bash
curl -i --json '{"name":"Alice","email":"Alice@Example.com","password":"secret123"}' localhost:8080/api/v1/auth/register
```
```http
HTTP/1.1 409 Conflict
```
```json
{ "code": "UM40900", "message": "email already exists" }
```

**Missing fields.** All missing fields are reported at once:

```bash
curl -i --json '{"email":"bob@example.com"}' localhost:8080/api/v1/auth/register
```
```http
HTTP/1.1 422 Unprocessable Entity
```
```json
{ "code": "UM42200", "message": "name is required, password is required" }
```

**Invalid value:**

```bash
curl -i --json '{"name":"Bob","email":"not-an-email","password":"secret123"}' localhost:8080/api/v1/auth/register
```
```http
HTTP/1.1 422 Unprocessable Entity
```
```json
{ "code": "UM42200", "message": "invalid email address" }
```

Other value errors: `name must be 1-100 characters`, `password must be at least 8 characters`.

**Malformed JSON:**

```bash
curl -i --json '{"name":' localhost:8080/api/v1/auth/register
```
```http
HTTP/1.1 400 Bad Request
```
```json
{ "code": "UM40000", "message": "invalid JSON request body" }
```

### Login

`POST /api/v1/auth/login` (public)

```bash
curl -i --json '{"email":"alice@example.com","password":"secret123"}' localhost:8080/api/v1/auth/login
```
```http
HTTP/1.1 200 OK
```
```json
{
  "code": "UM00000",
  "message": "success",
  "data": {
    "accessToken": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJlbWFpbCI6ImFsaWNlQGV4YW1wbGUuY29tIiwiaXNzIjoidXNlci1tYW5hZ2VtZW50LWFwaSIsInN1YiI6IjAxYTFhMDQyLWRmZGMtNzc2NC1iNDcxLWRlMmQ4ZWIxNTc4MyIsImV4cCI6MTc5MzY5MzcwMCwibmJmIjoxNzkzNjkwMTAwLCJpYXQiOjE3OTM2OTAxMDB9.A9wn9T-p_3L88J_OyPCkbO4UJ6lKnD2mgV6DOTct3mQ",
    "tokenType": "Bearer",
    "expiresAt": "2026-11-03T08:15:00Z",
    "user": {
      "id": "01a1a042-dfdc-7764-b471-de2d8eb15783",
      "name": "Alice",
      "email": "alice@example.com",
      "createdAt": "2026-11-03T07:14:59.676Z",
      "updatedAt": "2026-11-03T07:14:59.676Z"
    }
  }
}
```

**Wrong password or unknown email.** Both get the same response, so it doesn't reveal which emails are registered:

```bash
curl -i --json '{"email":"alice@example.com","password":"wrong-password"}' localhost:8080/api/v1/auth/login
```
```http
HTTP/1.1 401 Unauthorized
WWW-Authenticate: Bearer realm="api"
```
```json
{ "code": "UM40101", "message": "invalid email or password" }
```

### Me

`GET /api/v1/auth/me` (JWT): returns the user the token belongs to.

```bash
curl -i localhost:8080/api/v1/auth/me -H "Authorization: Bearer $TOKEN"
```
```http
HTTP/1.1 200 OK
```
```json
{
  "code": "UM00000",
  "message": "success",
  "data": {
    "id": "01a1a042-dfdc-7764-b471-de2d8eb15783",
    "name": "Alice",
    "email": "alice@example.com",
    "createdAt": "2026-11-03T07:14:59.676Z",
    "updatedAt": "2026-11-03T07:14:59.676Z"
  }
}
```

**No token, or an invalid one.** This applies to every protected route:

```bash
curl -i localhost:8080/api/v1/users
```
```http
HTTP/1.1 401 Unauthorized
WWW-Authenticate: Bearer realm="api"
```
```json
{ "code": "UM40100", "message": "missing or malformed Authorization header" }
```

```bash
curl -i localhost:8080/api/v1/users -H "Authorization: Bearer abc.def.ghi"
```
```http
HTTP/1.1 401 Unauthorized
WWW-Authenticate: Bearer realm="api"
```
```json
{ "code": "UM40100", "message": "invalid or expired token" }
```

### List users

`GET /api/v1/users?page=0&pageSize=10` (JWT). `page` is 0-based. `pageSize` defaults to 20 and is capped at 100. Users are sorted oldest first, and `total` counts every user, not just this page.

```bash
curl -i 'localhost:8080/api/v1/users?page=0&pageSize=10' -H "Authorization: Bearer $TOKEN"
```
```http
HTTP/1.1 200 OK
```
```json
{
  "code": "UM00000",
  "message": "success",
  "data": {
    "users": [
      {
        "id": "01a1a042-dfdc-7764-b471-de2d8eb15783",
        "name": "Alice",
        "email": "alice@example.com",
        "createdAt": "2026-11-03T07:14:59.676Z",
        "updatedAt": "2026-11-03T07:14:59.676Z"
      },
      {
        "id": "01a1a042-e0e1-73ee-893d-199bf0b5f982",
        "name": "Bob",
        "email": "bob@example.com",
        "createdAt": "2026-11-03T07:14:59.937Z",
        "updatedAt": "2026-11-03T07:14:59.937Z"
      }
    ],
    "total": 2
  }
}
```

### Get user

`GET /api/v1/users/:id` (JWT)

```bash
curl -i localhost:8080/api/v1/users/$ID -H "Authorization: Bearer $TOKEN"
```
```http
HTTP/1.1 200 OK
```
```json
{
  "code": "UM00000",
  "message": "success",
  "data": {
    "id": "01a1a042-dfdc-7764-b471-de2d8eb15783",
    "name": "Alice",
    "email": "alice@example.com",
    "createdAt": "2026-11-03T07:14:59.676Z",
    "updatedAt": "2026-11-03T07:14:59.676Z"
  }
}
```

**Unknown id:**

```bash
curl -i localhost:8080/api/v1/users/0190f0f0-0000-7000-8000-000000000000 -H "Authorization: Bearer $TOKEN"
```
```http
HTTP/1.1 404 Not Found
```
```json
{ "code": "UM40401", "message": "user not found" }
```

### Update user

`PATCH /api/v1/users/:id` (JWT). This is a partial update: send `name`, `email`, or both. Omitted fields are left unchanged, and `updatedAt` is bumped.

```bash
curl -i -X PATCH --json '{"name":"Alicia"}' localhost:8080/api/v1/users/$ID -H "Authorization: Bearer $TOKEN"
```
```http
HTTP/1.1 200 OK
```
```json
{
  "code": "UM00000",
  "message": "success",
  "data": {
    "id": "01a1a042-dfdc-7764-b471-de2d8eb15783",
    "name": "Alicia",
    "email": "alice@example.com",
    "createdAt": "2026-11-03T07:14:59.676Z",
    "updatedAt": "2026-11-03T07:15:00.242Z"
  }
}
```

**Nothing to update:**

```bash
curl -i -X PATCH --json '{}' localhost:8080/api/v1/users/$ID -H "Authorization: Bearer $TOKEN"
```
```http
HTTP/1.1 422 Unprocessable Entity
```
```json
{ "code": "UM42200", "message": "at least one of name or email is required" }
```

**Email taken by another user:**

```bash
curl -i -X PATCH --json '{"email":"bob@example.com"}' localhost:8080/api/v1/users/$ID -H "Authorization: Bearer $TOKEN"
```
```http
HTTP/1.1 409 Conflict
```
```json
{ "code": "UM40900", "message": "email already exists" }
```

### Delete user

`DELETE /api/v1/users/:id` (JWT). This is a hard delete, and the email can be registered again afterwards. An unknown id returns `404` `UM40401`.

```bash
curl -i -X DELETE localhost:8080/api/v1/users/$OTHER_ID -H "Authorization: Bearer $TOKEN"
```
```http
HTTP/1.1 200 OK
```
```json
{ "code": "UM00000", "message": "success" }
```

### Health

`GET /health` (public) pings MongoDB.

```bash
curl -i localhost:8080/health
```
```http
HTTP/1.1 200 OK
```
```json
{ "code": "UM00000", "message": "success" }
```

When MongoDB doesn't answer within 2 seconds:

```http
HTTP/1.1 503 Service Unavailable
```
```json
{ "code": "UM50300", "message": "database not connected" }
```

### Routing errors

```bash
curl -i localhost:8080/nope
```
```http
HTTP/1.1 404 Not Found
```
```json
{ "code": "UM40400", "message": "not found" }
```

```bash
curl -i -X PUT localhost:8080/api/v1/auth/login
```
```http
HTTP/1.1 405 Method Not Allowed
```
```json
{ "code": "UM40500", "message": "method not allowed" }
```

Any unexpected server error returns `500` `{"code":"UM50000","message":"internal error"}`. The real cause is written to the log, not sent to the client.

## gRPC

gRPC has no envelope: success returns the response message, and failures return a gRPC status. The examples use [grpcurl](https://github.com/fullstorydev/grpcurl), which works without `.proto` files because the server has reflection enabled (`server.grpc_reflection: true`).

> grpcurl prints JSON in proto3's JSON mapping: `int64` fields like `total` appear as strings (`"2"`), and field names are camelCase (`pageSize`), although grpcurl also accepts the proto names (`page_size`) in requests.

**Register** (`auth.v1.AuthService/Register`, public)

```bash
grpcurl -plaintext -d '{"name":"Carol","email":"carol@example.com","password":"secret123"}' \
  localhost:50051 auth.v1.AuthService/Register
```
```json
{
  "user": {
    "id": "01a1a042-e2a0-70a5-b43a-1d8ef7a21df3",
    "name": "Carol",
    "email": "carol@example.com",
    "createdAt": "2026-11-03T07:15:00.384Z",
    "updatedAt": "2026-11-03T07:15:00.384Z"
  }
}
```

**Login** (`auth.v1.AuthService/Login`, public)

```bash
grpcurl -plaintext -d '{"email":"carol@example.com","password":"secret123"}' \
  localhost:50051 auth.v1.AuthService/Login
```
```json
{
  "accessToken": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJlbWFpbCI6ImNhcm9sQGV4YW1wbGUuY29tIiwiaXNzIjoidXNlci1tYW5hZ2VtZW50LWFwaSIsInN1YiI6IjAxYTFhMDQyLWUyYTAtNzBhNS1iNDNhLTFkOGVmN2EyMWRmMyIsImV4cCI6MTc5MzY5MzcwMCwibmJmIjoxNzkzNjkwMTAwLCJpYXQiOjE3OTM2OTAxMDB9.y35iYmkX-QWweqEbbz8A_S-RTQOkw8kt0uqs_42H0X8",
  "tokenType": "Bearer",
  "expiresAt": "2026-11-03T08:15:00Z",
  "user": {
    "id": "01a1a042-e2a0-70a5-b43a-1d8ef7a21df3",
    "name": "Carol",
    "email": "carol@example.com",
    "createdAt": "2026-11-03T07:15:00.384Z",
    "updatedAt": "2026-11-03T07:15:00.384Z"
  }
}
```

**List users** (`user.v1.UserService/ListUsers`, JWT)

```bash
grpcurl -plaintext -H "authorization: Bearer $TOKEN" -d '{"page":0,"page_size":2}' \
  localhost:50051 user.v1.UserService/ListUsers
```
```json
{
  "users": [
    {
      "id": "01a1a042-dfdc-7764-b471-de2d8eb15783",
      "name": "Alicia",
      "email": "alice@example.com",
      "createdAt": "2026-11-03T07:14:59.676Z",
      "updatedAt": "2026-11-03T07:15:00.242Z"
    },
    {
      "id": "01a1a042-e2a0-70a5-b43a-1d8ef7a21df3",
      "name": "Carol",
      "email": "carol@example.com",
      "createdAt": "2026-11-03T07:15:00.384Z",
      "updatedAt": "2026-11-03T07:15:00.384Z"
    }
  ],
  "total": "2"
}
```

`GetUser` and `UpdateUser` return `{"user": {...}}`. `DeleteUser` returns `{}`.

**Errors**

```bash
# no token
grpcurl -plaintext -d '{"id":"'$ID'"}' localhost:50051 user.v1.UserService/GetUser
```
```
ERROR:
  Code: Unauthenticated
  Message: missing authorization metadata
```

```bash
# unknown id
grpcurl -plaintext -H "authorization: Bearer $TOKEN" -d '{"id":"nope"}' localhost:50051 user.v1.UserService/GetUser
```
```
ERROR:
  Code: NotFound
  Message: user not found
```

```bash
# update with no fields
grpcurl -plaintext -H "authorization: Bearer $TOKEN" -d '{"id":"'$ID'"}' localhost:50051 user.v1.UserService/UpdateUser
```
```
ERROR:
  Code: InvalidArgument
  Message: at least one of name or email is required
```

| Business error | REST | gRPC code |
|---|---|---|
| Validation (missing/invalid field) | 422 `UM42200` | `InvalidArgument` |
| User not found | 404 `UM40401` | `NotFound` |
| Email already exists | 409 `UM40900` | `AlreadyExists` |
| Invalid credentials | 401 `UM40101` | `Unauthenticated` |
| Missing/invalid token | 401 `UM40100` | `Unauthenticated` |
| Anything else | 500 `UM50000` | `Internal` |
