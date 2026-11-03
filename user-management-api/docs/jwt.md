# JWT guide

This guide covers how the API issues JWT access tokens, how to send them, and what to do when one is rejected. The same tokens work for REST and gRPC.

## 1. Configure the signing secret

The server signs tokens with **HS256** (HMAC-SHA256), using `jwt.secret` from `config/config.yaml` as the key. The server won't start without a secret, or with one shorter than 32 bytes:

```bash
cp config/config.example.yaml config/config.yaml
openssl rand -base64 48        # paste the output into jwt.secret
```

```yaml
jwt:
  secret: "kX3v...your-48-random-bytes...Q9"
  issuer: user-management-api   # written to and checked against the iss claim
  ttl: 1h                       # token lifetime (Go duration: 15m, 1h, 24h)
```

In containers and CI, pass the secret as an environment variable instead of writing it to a file:

```bash
JWT_SECRET="$(openssl rand -base64 48)" make run
```

> Anyone who knows the secret can create valid tokens for any user, so treat it like a password. Changing the secret invalidates every token already issued, which forces everyone to log in again.

## 2. Get a token

Register once, then log in. Only login returns a token; register just creates the account.

```bash
curl --json '{"name":"Alice","email":"alice@example.com","password":"secret123"}' \
  localhost:8080/api/v1/auth/register

curl --json '{"email":"alice@example.com","password":"secret123"}' \
  localhost:8080/api/v1/auth/login
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

Save it in a shell variable for the rest of the session:

```bash
TOKEN=$(curl -s --json '{"email":"alice@example.com","password":"secret123"}' \
  localhost:8080/api/v1/auth/login | jq -r .data.accessToken)
```

Over gRPC, the token is in the `accessToken` field of the `Login` response:

```bash
TOKEN=$(grpcurl -plaintext -d '{"email":"alice@example.com","password":"secret123"}' \
  localhost:50051 auth.v1.AuthService/Login | jq -r .accessToken)
```

## 3. Use the token

Send the token on every protected call. The scheme `Bearer` is case-insensitive.

| Transport | Where | Example |
|---|---|---|
| REST | `Authorization` header | `curl localhost:8080/api/v1/auth/me -H "Authorization: Bearer $TOKEN"` |
| gRPC | `authorization` metadata | `grpcurl -plaintext -H "authorization: Bearer $TOKEN" -d '{"page":0}' localhost:50051 user.v1.UserService/ListUsers` |

`GET /api/v1/auth/me` is the quickest way to check a token: it returns the user the token belongs to.

**Public routes** (no token needed): `POST /api/v1/auth/register`, `POST /api/v1/auth/login`, `GET /health`, `auth.v1.AuthService/*`, gRPC health and gRPC reflection. **Everything else needs a token.** That covers all of `/api/v1/users`, `/api/v1/auth/me` and every `user.v1.UserService` RPC. The gRPC interceptor denies by default, so a newly added RPC is protected unless someone adds it to the public list.

## 4. What's inside a token

A JWT has three base64url parts: `header.payload.signature`. You can decode the first two without the secret, so never put anything private in them:

```bash
echo "$TOKEN" | cut -d. -f1 | base64 -d 2>/dev/null; echo
echo "$TOKEN" | cut -d. -f2 | tr '_-' '/+' | base64 -d 2>/dev/null; echo
```

```json
{"alg":"HS256","typ":"JWT"}
{"email":"alice@example.com","iss":"user-management-api","sub":"01a1a042-dfdc-7764-b471-de2d8eb15783","exp":1793693700,"nbf":1793690100,"iat":1793690100}
```

| Claim | Meaning |
|---|---|
| `sub` | User ID: who the token is for |
| `email` | The user's email when the token was issued (informational; it isn't updated if the email changes) |
| `iss` | Issuer, from `jwt.issuer` |
| `iat` / `nbf` | Issued-at / not-before (Unix seconds) |
| `exp` | Expiry (Unix seconds): `iat + jwt.ttl` |

The signature is HMAC-SHA256 over `header.payload`, keyed with `jwt.secret`. Editing any claim, for example changing `sub` to another user's id, breaks the signature and the token is rejected.

## 5. How the server verifies a token

The REST middleware (`middleware.ValidateJwt`) and the gRPC interceptor (`AuthUnaryInterceptor`) both call `JwtToken.Verify` in `app/adapters/security/jwt_token.go`. A token is accepted only if **all** of these hold:

1. The header is `Bearer <token>` and the token isn't empty.
2. `alg` is exactly `HS256`. `none`, `RS256` and every other algorithm are rejected, which blocks the "alg: none" and algorithm-confusion attacks.
3. The signature matches `jwt.secret`.
4. `iss` equals `jwt.issuer`.
5. `exp` is present and in the future, and `iat`/`nbf` aren't in the future. 30 seconds of clock skew is allowed for both.
6. `sub` isn't empty.

Once a token passes, its claims are available to the handler: `middleware.ClaimsFrom(c)` in REST and `ClaimsFromContext(ctx)` in gRPC.

## 6. Errors

| Situation | REST | gRPC |
|---|---|---|
| No `Authorization` header, or not `Bearer <token>` | `401` `UM40100` `missing or malformed Authorization header` | `Unauthenticated` `missing authorization metadata` / `authorization must be "Bearer <token>"` |
| Bad signature, wrong `alg`, wrong issuer, expired, garbage | `401` `UM40100` `invalid or expired token` | `Unauthenticated` `invalid or expired token` |
| Wrong email or password at login | `401` `UM40101` `invalid email or password` | `Unauthenticated` `invalid email or password` |

Every REST `401` also carries `WWW-Authenticate: Bearer realm="api"`. The reasons a token can be invalid all get one message on purpose, so an attacker can't learn which check failed.

**What to do with a 401:** call `login` again for a fresh token. There's no refresh token; see below.

## 7. Limits of this design

- **No refresh tokens.** When the access token expires, the client logs in again. Keep `jwt.ttl` short (15m–1h) and add refresh tokens if clients need long sessions.
- **No revocation.** Tokens are stateless: the server never stores them, so it can't cancel one early. Deleting a user or changing their email doesn't invalidate tokens already issued; they work until `exp`. The ways to cut every session are rotating `jwt.secret`, which logs out everyone, or adding a denylist store.
- **No roles.** A valid token proves who the caller is, but not what they may do. Any authenticated user can update or delete any user (see [design-decisions.md](design-decisions.md)).
- **One shared key.** HS256 uses the same secret to sign and verify, so every service that verifies tokens can also mint them. If other services need to verify tokens, switch to an asymmetric algorithm (RS256/EdDSA) and publish only the public key.
