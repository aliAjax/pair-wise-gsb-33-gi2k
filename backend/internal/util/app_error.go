package util

import "fmt"

// AppError is a business error carrying an HTTP status and a business code.
type AppError struct {
	HTTPStatus int
	Code       int
	Message    string
	// Details is serialized as the "details" field of the error envelope
	// when non-nil (e.g. stale-revision conflict metadata).
	Details any
}

func (e *AppError) Error() string {
	return fmt.Sprintf("code=%d message=%s", e.Code, e.Message)
}

// NewAppError builds an AppError with the given status/code/message.
func NewAppError(httpStatus, code int, message string) *AppError {
	return &AppError{HTTPStatus: httpStatus, Code: code, Message: message}
}

// WithDetails attaches structured details and returns the same error.
func (e *AppError) WithDetails(details any) *AppError {
	e.Details = details
	return e
}
