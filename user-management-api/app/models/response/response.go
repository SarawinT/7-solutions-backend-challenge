// Package response holds the REST response envelope and response models.
//
// Every REST response has the shape:
//
//	{"code": "UM00000", "message": "success", "data": {...}}
package response

import (
	"errors"

	"github.com/gofiber/fiber/v3"

	"github.com/SarawinT/user-management-api/app/constants"
	"github.com/SarawinT/user-management-api/app/models/entity"
)

type Response struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// SendResponse writes the envelope with the given HTTP status.
func (r Response) SendResponse(c fiber.Ctx, httpStatus int) error {
	return c.Status(httpStatus).JSON(r)
}

// SendSuccessResponse writes a success envelope (code UM00000).
func SendSuccessResponse(c fiber.Ctx, httpStatus int, data any) error {
	return Response{
		Code:    constants.CodeSuccess,
		Message: constants.ResponseMessage(constants.CodeSuccess),
		Data:    data,
	}.SendResponse(c, httpStatus)
}

// SendErrorResponse writes an error envelope.
func SendErrorResponse(c fiber.Ctx, httpStatus int, code, message string) error {
	return Response{Code: code, Message: message}.SendResponse(c, httpStatus)
}

// FromError maps an error to its HTTP status, response code and message.
// Unknown errors become a generic 500 so internal details never leak.
func FromError(err error) (httpStatus int, code, message string) {
	if ve, ok := errors.AsType[*entity.ValidationError](err); ok {
		return fiber.StatusUnprocessableEntity, constants.CodeValidationFailed, ve.Message
	}

	switch {
	case errors.Is(err, entity.ErrUserNotFound):
		return fiber.StatusNotFound, constants.CodeUserNotFound, err.Error()
	case errors.Is(err, entity.ErrEmailAlreadyExists):
		return fiber.StatusConflict, constants.CodeEmailAlreadyExists, err.Error()
	case errors.Is(err, entity.ErrInvalidName),
		errors.Is(err, entity.ErrInvalidEmail),
		errors.Is(err, entity.ErrInvalidPassword):
		return fiber.StatusUnprocessableEntity, constants.CodeValidationFailed, err.Error()
	case errors.Is(err, entity.ErrInvalidCredentials):
		return fiber.StatusUnauthorized, constants.CodeInvalidCredentials, err.Error()
	case errors.Is(err, entity.ErrInvalidToken):
		return fiber.StatusUnauthorized, constants.CodeUnauthorized, err.Error()
	}

	// Framework errors: malformed body, unknown route, wrong method, ...
	if fe, ok := errors.AsType[*fiber.Error](err); ok {
		switch fe.Code {
		case fiber.StatusBadRequest:
			return fe.Code, constants.CodeBadRequest, fe.Message
		case fiber.StatusUnauthorized:
			return fe.Code, constants.CodeUnauthorized, fe.Message
		case fiber.StatusNotFound:
			return fe.Code, constants.CodeNotFound, constants.ResponseMessage(constants.CodeNotFound)
		case fiber.StatusMethodNotAllowed:
			return fe.Code, constants.CodeMethodNotAllowed, constants.ResponseMessage(constants.CodeMethodNotAllowed)
		}
		// Any other client error (413 body too large, 422, ...) keeps its
		// status rather than becoming a 500.
		if fe.Code >= 400 && fe.Code < 500 {
			return fe.Code, constants.CodeBadRequest, fe.Message
		}
	}
	return fiber.StatusInternalServerError, constants.CodeInternalError, constants.ResponseMessage(constants.CodeInternalError)
}
