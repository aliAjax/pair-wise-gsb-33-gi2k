package constants

// Business error codes. Code 0 means success.
const (
	CodeSuccess         = 0
	CodeBadRequest      = 40000
	CodeUnauthorized    = 40100
	CodeForbidden       = 40300
	CodeNotFound        = 40400
	CodeConflict        = 40900
	CodeRateLimited     = 42900
	CodeValidationError = 42200
	CodeRevisionStale   = 40901
	CodeInternalError   = 50000
)

// Error messages referenced across handlers and middleware.
const (
	MsgInvalidParam  = "invalid request parameter"
	MsgUnauthorized  = "authentication required"
	MsgForbidden     = "permission denied"
	MsgNotFound      = "resource not found"
	MsgConflict      = "resource conflict"
	MsgRevisionStale = "online article has a newer revision, please re-merge before publishing"
	MsgRateLimited   = "too many requests"
	MsgInternalError = "internal server error"
)
