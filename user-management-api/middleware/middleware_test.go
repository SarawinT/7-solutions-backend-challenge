package middleware_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/SarawinT/user-management-api/middleware"
)

// newLoggedApp mirrors the production middleware order (logger outermost,
// then recover) with a few routes covering success, slow, error and panic.
func newLoggedApp(buf *bytes.Buffer) *fiber.App {
	log := slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	m := middleware.New(log, nil)

	app := fiber.New(fiber.Config{ErrorHandler: m.ErrorHandler})
	app.Use(m.AttachRequestLogger())
	app.Use(m.Recover())

	app.Get("/ok", func(c fiber.Ctx) error { return c.SendString("ok") })
	app.Post("/slow", func(c fiber.Ctx) error {
		time.Sleep(20 * time.Millisecond)
		return c.SendStatus(fiber.StatusCreated)
	})
	app.Get("/fail", func(c fiber.Ctx) error { return fiber.ErrUnprocessableEntity })
	app.Get("/unauthorized", m.ValidateJwt(), func(c fiber.Ctx) error { return nil }) // no token sent
	app.Get("/panic", func(c fiber.Ctx) error { panic("boom") })
	return app
}

func lastLogLine(t *testing.T, buf *bytes.Buffer) map[string]any {
	t.Helper()
	lines := bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n"))
	var entry map[string]any
	if err := json.Unmarshal(lines[len(lines)-1], &entry); err != nil {
		t.Fatalf("log line is not JSON: %q", lines[len(lines)-1])
	}
	return entry
}

func TestRequestLogger(t *testing.T) {
	tests := []struct {
		method, path string
		wantStatus   int
		wantLevel    string
		minDuration  float64 // ms
	}{
		{http.MethodGet, "/ok", 200, "INFO", 0},
		{http.MethodPost, "/slow", 201, "INFO", 20},
		{http.MethodGet, "/fail", 422, "WARN", 0},
		{http.MethodGet, "/unauthorized", 401, "WARN", 0},
		{http.MethodGet, "/missing", 404, "WARN", 0},
		{http.MethodGet, "/panic", 500, "ERROR", 0}, // must still be logged
	}
	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			var buf bytes.Buffer
			app := newLoggedApp(&buf)

			resp, err := app.Test(httptest.NewRequest(tt.method, tt.path, nil))
			if err != nil {
				t.Fatal(err)
			}
			resp.Body.Close()
			if resp.StatusCode != tt.wantStatus {
				t.Fatalf("response status %d, want %d", resp.StatusCode, tt.wantStatus)
			}

			entry := lastLogLine(t, &buf)
			if entry["msg"] != "http request" || entry["level"] != tt.wantLevel {
				t.Errorf("msg/level = %v/%v, want http request/%s", entry["msg"], entry["level"], tt.wantLevel)
			}
			if entry["method"] != tt.method || entry["path"] != tt.path {
				t.Errorf("method/path = %v %v, want %s %s", entry["method"], entry["path"], tt.method, tt.path)
			}
			if entry["status"] != float64(tt.wantStatus) {
				t.Errorf("logged status %v, want %d", entry["status"], tt.wantStatus)
			}
			d, ok := entry["duration_ms"].(float64)
			if !ok || d < tt.minDuration {
				t.Errorf("duration_ms = %v, want a number >= %v", entry["duration_ms"], tt.minDuration)
			}
		})
	}
}
