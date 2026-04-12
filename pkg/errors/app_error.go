package apperrors

import "fmt"

type Code string

const (
	CodeInvalidArgument Code = "INVALID_ARGUMENT"
	CodeNotFound        Code = "NOT_FOUND"
	CodeInternal        Code = "INTERNAL"
	CodeUnauthorized    Code = "UNAUTHORIZED"
)

type AppError struct {
	Code    Code
	Message string
	Err     error
}

func (e *AppError) Error() string {
	if e.Err == nil {
		return e.Message
	}
	return fmt.Sprintf("%s: %v", e.Message, e.Err)
}

func (e *AppError) Unwrap() error {
	return e.Err
}

func New(code Code, message string, err error) *AppError {
	return &AppError{Code: code, Message: message, Err: err}
}

func InvalidArgument(message string) *AppError {
	return New(CodeInvalidArgument, message, nil)
}

func NotFound(message string) *AppError {
	return New(CodeNotFound, message, nil)
}

func Internal(message string, err error) *AppError {
	return New(CodeInternal, message, err)
}

func Unauthorized(message string) *AppError {
	return New(CodeUnauthorized, message, nil)
}

func CodeOf(err error) Code {
	if err == nil {
		return ""
	}
	appErr, ok := err.(*AppError)
	if !ok {
		return CodeInternal
	}
	return appErr.Code
}
