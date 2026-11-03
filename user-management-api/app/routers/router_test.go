package routers_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/SarawinT/user-management-api/app/adapters/repository/mongodb"
	"github.com/SarawinT/user-management-api/app/adapters/repository/mongodb/mongotest"
	"github.com/SarawinT/user-management-api/app/adapters/security"
	"github.com/SarawinT/user-management-api/app/constants"
	"github.com/SarawinT/user-management-api/app/routers"
	"github.com/SarawinT/user-management-api/middleware"
)

// End-to-end REST tests: real routers, middleware, services and MongoDB
// (needs MONGO_TEST_URI, see mongotest).

const testSecret = "test-secret-test-secret-test-secret!"

// Real bcrypt can exceed Fiber's default 1s test timeout under -race.
var testCfg = fiber.TestConfig{Timeout: 10 * time.Second}

func newApp(t *testing.T) *fiber.App {
	t.Helper()
	db := mongotest.NewDatabase(t)
	if err := mongodb.EnsureUserIndexes(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	token, err := security.NewJwtToken(testSecret, "test", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	m := middleware.New(slog.New(slog.NewTextHandler(io.Discard, nil)), token)

	app := fiber.New(fiber.Config{ErrorHandler: m.ErrorHandler})
	app.Use(m.AttachRequestLogger())
	app.Use(m.Recover())
	routers.AuthRouter(app, m, db, token)
	routers.UserRouter(app, m, db)
	return app
}

type envelope struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Data    map[string]any `json:"data"`
}

// do sends a request with an optional bearer token and decodes the envelope.
func do(t *testing.T, app *fiber.App, method, path, token, body string) (*http.Response, envelope) {
	t.Helper()
	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, r)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := app.Test(req, testCfg)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var env envelope
	if b, _ := io.ReadAll(resp.Body); len(b) > 0 {
		if err := json.Unmarshal(b, &env); err != nil {
			t.Fatalf("response is not an envelope %q: %v", b, err)
		}
	}
	return resp, env
}

// expect asserts the HTTP status and response code.
func expect(t *testing.T, step string, resp *http.Response, env envelope, status int, code string) {
	t.Helper()
	if resp.StatusCode != status || env.Code != code {
		t.Fatalf("%s: got %d %s %q, want %d %s", step, resp.StatusCode, env.Code, env.Message, status, code)
	}
}

func registerAndLogin(t *testing.T, app *fiber.App, email string) string {
	t.Helper()
	resp, env := do(t, app, http.MethodPost, "/api/v1/auth/register", "",
		`{"name":"Test","email":"`+email+`","password":"secret123"}`)
	expect(t, "register", resp, env, http.StatusCreated, constants.CodeSuccess)

	resp, env = do(t, app, http.MethodPost, "/api/v1/auth/login", "",
		`{"email":"`+email+`","password":"secret123"}`)
	expect(t, "login", resp, env, http.StatusOK, constants.CodeSuccess)
	return env.Data["accessToken"].(string)
}

func TestUsersAPI(t *testing.T) {
	app := newApp(t)
	token := registerAndLogin(t, app, "admin@example.com")

	// Users are created only via /auth/register (public).
	resp, env := do(t, app, http.MethodPost, "/api/v1/auth/register", "",
		`{"name":"Alice","email":"alice@example.com","password":"secret123"}`)
	expect(t, "register alice", resp, env, http.StatusCreated, constants.CodeSuccess)
	if _, leaked := env.Data["passwordHash"]; leaked {
		t.Fatal("password hash leaked in response")
	}
	if env.Data["createdAt"] == nil || resp.Header.Get("Location") == "" {
		t.Fatalf("register: missing createdAt or Location: %v", env.Data)
	}
	id := env.Data["id"].(string)

	resp, env = do(t, app, http.MethodPost, "/api/v1/auth/register", "",
		`{"name":"Bob","email":"alice@example.com","password":"secret123"}`)
	expect(t, "duplicate email", resp, env, http.StatusConflict, constants.CodeEmailAlreadyExists)

	resp, env = do(t, app, http.MethodPost, "/api/v1/auth/register", "",
		`{"name":"Bob","email":"bad","password":"secret123"}`)
	expect(t, "invalid email", resp, env, http.StatusUnprocessableEntity, constants.CodeValidationFailed)

	resp, env = do(t, app, http.MethodPost, "/api/v1/auth/register", "", `{}`)
	expect(t, "missing fields", resp, env, http.StatusUnprocessableEntity, constants.CodeValidationFailed)
	if env.Message != "name is required, email is required, password is required" {
		t.Fatalf("missing fields message: %q", env.Message)
	}

	resp, env = do(t, app, http.MethodPost, "/api/v1/auth/register", "", `{not json`)
	expect(t, "malformed body", resp, env, http.StatusBadRequest, constants.CodeBadRequest)

	// POST /users does not exist.
	if resp, _ := do(t, app, http.MethodPost, "/api/v1/users", token, `{}`); resp.StatusCode != http.StatusNotFound && resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("POST /users: got %d, want 404 or 405", resp.StatusCode)
	}

	resp, env = do(t, app, http.MethodPatch, "/api/v1/users/"+id, token, `{"name":"Alicia"}`)
	expect(t, "update", resp, env, http.StatusOK, constants.CodeSuccess)
	if env.Data["name"] != "Alicia" || env.Data["email"] != "alice@example.com" {
		t.Fatalf("update: unexpected data %v", env.Data)
	}

	resp, env = do(t, app, http.MethodPatch, "/api/v1/users/"+id, token, `{}`)
	expect(t, "update with no fields", resp, env, http.StatusUnprocessableEntity, constants.CodeValidationFailed)

	resp, env = do(t, app, http.MethodGet, "/api/v1/users?page=0&pageSize=10", token, "")
	expect(t, "list", resp, env, http.StatusOK, constants.CodeSuccess)
	if env.Data["total"] != float64(2) {
		t.Fatalf("list: unexpected data %v", env.Data)
	}

	resp, env = do(t, app, http.MethodGet, "/api/v1/users/"+id, token, "")
	expect(t, "get", resp, env, http.StatusOK, constants.CodeSuccess)

	resp, env = do(t, app, http.MethodDelete, "/api/v1/users/"+id, token, "")
	expect(t, "delete", resp, env, http.StatusOK, constants.CodeSuccess)
	if env.Data != nil {
		t.Fatalf("delete: expected no data, got %v", env.Data)
	}

	resp, env = do(t, app, http.MethodGet, "/api/v1/users/"+id, token, "")
	expect(t, "get deleted", resp, env, http.StatusNotFound, constants.CodeUserNotFound)

	resp, env = do(t, app, http.MethodGet, "/api/v1/nope", token, "")
	expect(t, "unknown route", resp, env, http.StatusNotFound, constants.CodeNotFound)
}

func TestAuthAPI(t *testing.T) {
	app := newApp(t)

	t.Run("protected routes reject missing or bad tokens", func(t *testing.T) {
		for name, token := range map[string]string{"missing": "", "garbage": "not-a-jwt"} {
			resp, env := do(t, app, http.MethodGet, "/api/v1/users", token, "")
			expect(t, name+" token", resp, env, http.StatusUnauthorized, constants.CodeUnauthorized)
			if resp.Header.Get("WWW-Authenticate") == "" {
				t.Fatalf("%s token: missing WWW-Authenticate header", name)
			}
		}

		req := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
		req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
		resp, err := app.Test(req, testCfg)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("basic auth: got %d", resp.StatusCode)
		}

		other, _ := security.NewJwtToken("another-secret-another-secret-another", "test", time.Hour)
		forged, _, _ := other.Issue("someone", "x@example.com")
		resp, env := do(t, app, http.MethodGet, "/api/v1/users", forged, "")
		expect(t, "forged token", resp, env, http.StatusUnauthorized, constants.CodeUnauthorized)
	})

	token := registerAndLogin(t, app, "alice@example.com")

	t.Run("me returns the token's user", func(t *testing.T) {
		resp, env := do(t, app, http.MethodGet, "/api/v1/auth/me", token, "")
		expect(t, "me", resp, env, http.StatusOK, constants.CodeSuccess)
		if env.Data["email"] != "alice@example.com" {
			t.Fatalf("me: unexpected data %v", env.Data)
		}
		resp, env = do(t, app, http.MethodGet, "/api/v1/auth/me", "", "")
		expect(t, "me without token", resp, env, http.StatusUnauthorized, constants.CodeUnauthorized)
	})

	t.Run("login response", func(t *testing.T) {
		resp, env := do(t, app, http.MethodPost, "/api/v1/auth/login", "",
			`{"email":"alice@example.com","password":"secret123"}`)
		expect(t, "login", resp, env, http.StatusOK, constants.CodeSuccess)
		if env.Data["tokenType"] != "Bearer" || env.Data["expiresAt"] == nil || env.Data["user"] == nil {
			t.Fatalf("unexpected data %v", env.Data)
		}
		if resp.Header.Get("Cache-Control") != "no-store" {
			t.Fatal("token response must not be cacheable")
		}
	})

	t.Run("bad credentials are indistinguishable", func(t *testing.T) {
		var msgs []string
		for _, b := range []string{
			`{"email":"alice@example.com","password":"wrong-password"}`,
			`{"email":"nobody@example.com","password":"secret123"}`,
		} {
			resp, env := do(t, app, http.MethodPost, "/api/v1/auth/login", "", b)
			expect(t, "bad credentials", resp, env, http.StatusUnauthorized, constants.CodeInvalidCredentials)
			msgs = append(msgs, env.Message)
		}
		if msgs[0] != msgs[1] {
			t.Fatalf("error messages differ: %v", msgs)
		}
	})
}
