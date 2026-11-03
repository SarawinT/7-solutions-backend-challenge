package constants

// ContextKeyClaims is the Fiber Locals key (and gRPC context key) under
// which the authenticated caller's token claims are stored.
const ContextKeyClaims = "auth.claims"

// AuthorizationHeader and BearerScheme describe the expected token header:
// "Authorization: Bearer <token>".
const (
	AuthorizationHeader = "Authorization"
	BearerScheme        = "Bearer"
)
