package constants

// Response codes returned in the "code" field of every REST response
// envelope. Clients should branch on these rather than on message text.
const (
	CodeSuccess            = "UM00000"
	CodeBadRequest         = "UM40000" // malformed JSON body
	CodeUnauthorized       = "UM40100" // missing, malformed, invalid or expired token
	CodeInvalidCredentials = "UM40101" // wrong email or password
	CodeNotFound           = "UM40400" // unknown route
	CodeUserNotFound       = "UM40401"
	CodeMethodNotAllowed   = "UM40500"
	CodeEmailAlreadyExists = "UM40900"
	CodeValidationFailed   = "UM42200" // missing or invalid field
	CodeInternalError      = "UM50000"
	CodeServiceUnavailable = "UM50300" // health check: database unreachable
)

var responseMessages = map[string]string{
	CodeSuccess:            "success",
	CodeBadRequest:         "invalid JSON request body",
	CodeUnauthorized:       "missing or invalid token",
	CodeInvalidCredentials: "invalid email or password",
	CodeNotFound:           "not found",
	CodeUserNotFound:       "user not found",
	CodeMethodNotAllowed:   "method not allowed",
	CodeEmailAlreadyExists: "email already exists",
	CodeValidationFailed:   "validation failed",
	CodeInternalError:      "internal error",
	CodeServiceUnavailable: "database not connected",
}

// ResponseMessage returns the default English message for a code.
func ResponseMessage(code string) string {
	if msg, ok := responseMessages[code]; ok {
		return msg
	}
	return responseMessages[CodeInternalError]
}
