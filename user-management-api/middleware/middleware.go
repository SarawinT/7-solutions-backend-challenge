// Package middleware holds the HTTP (Fiber) middleware shared by all
// routes: request IDs, request logging, panic recovery, JWT validation and
// the error handler that renders the response envelope.
package middleware

import (
	"log/slog"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	recoverer "github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/gofiber/fiber/v3/middleware/requestid"

	"github.com/SarawinT/user-management-api/app/constants"
	"github.com/SarawinT/user-management-api/app/core/ports"
	"github.com/SarawinT/user-management-api/app/models/entity"
	"github.com/SarawinT/user-management-api/app/models/response"
)

var errMissingToken = fiber.NewError(fiber.StatusUnauthorized, "missing or malformed Authorization header")

type Middleware struct {
	logger *slog.Logger
	token  ports.TokenPort
}

func New(logger *slog.Logger, token ports.TokenPort) *Middleware {
	return &Middleware{logger: logger, token: token}
}

// SetHeaderID assigns every request an X-Request-ID (or keeps the
// client's), echoed in the response and the request log.
func (m *Middleware) SetHeaderID() fiber.Handler {
	return requestid.New()
}

// Recover turns panics into 500 responses instead of crashing the server.
func (m *Middleware) Recover() fiber.Handler {
	return recoverer.New()
}

// AttachRequestLogger logs one line per request with the HTTP method, path,
// response status and execution time.
//
// Register it as the OUTERMOST middleware (before Recover) so every request
// is logged, including ones that panic or fail in other middleware. Errors
// are resolved through ErrorHandler here so the logged status matches the
// response the client receives.
func (m *Middleware) AttachRequestLogger() fiber.Handler {
	return func(c fiber.Ctx) error {
		start := time.Now()

		if err := c.Next(); err != nil {
			if hErr := m.ErrorHandler(c, err); hErr != nil {
				return hErr
			}
		}

		elapsed := time.Since(start)
		status := c.Response().StatusCode()

		level := slog.LevelInfo
		switch {
		case status >= fiber.StatusInternalServerError:
			level = slog.LevelError
		case status >= fiber.StatusBadRequest:
			level = slog.LevelWarn
		}

		m.logger.Log(c.Context(), level, "http request",
			slog.String("method", c.Method()),
			slog.String("path", c.Path()),
			slog.Int("status", status),
			slog.Float64("duration_ms", float64(elapsed.Microseconds())/1000),
			slog.String("request_id", requestid.FromContext(c)),
		)
		return nil
	}
}

// ValidateJwt requires "Authorization: Bearer <jwt>" and stores the token
// claims for the handler (see ClaimsFrom). Invalid requests get 401.
func (m *Middleware) ValidateJwt() fiber.Handler {
	return func(c fiber.Ctx) error {
		scheme, token, ok := strings.Cut(c.Get(constants.AuthorizationHeader), " ")
		token = strings.TrimSpace(token)
		if !ok || !strings.EqualFold(scheme, constants.BearerScheme) || token == "" {
			return errMissingToken
		}

		claims, err := m.token.Verify(token)
		if err != nil {
			return err
		}
		c.Locals(constants.ContextKeyClaims, claims)
		return c.Next()
	}
}

// ClaimsFrom returns the authenticated caller's claims. It is only set on
// routes behind ValidateJwt.
func ClaimsFrom(c fiber.Ctx) (*entity.TokenClaimsEntity, bool) {
	claims, ok := c.Locals(constants.ContextKeyClaims).(*entity.TokenClaimsEntity)
	return claims, ok
}

// ErrorHandler is Fiber's central error handler. Handlers return errors;
// this renders them as the response envelope with the right status and code.
func (m *Middleware) ErrorHandler(c fiber.Ctx, err error) error {
	status, code, msg := response.FromError(err)
	if status >= fiber.StatusInternalServerError {
		// Log the real cause; clients get a generic message.
		m.logger.ErrorContext(c.Context(), "unhandled error",
			"method", c.Method(), "path", c.Path(), "err", err)
	}
	if status == fiber.StatusUnauthorized {
		c.Set(fiber.HeaderWWWAuthenticate, `Bearer realm="api"`)
	}
	return response.SendErrorResponse(c, status, code, msg)
}
