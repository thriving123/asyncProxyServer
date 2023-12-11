package errors

import "fmt"

type BusinessError struct {
	Code       int
	Message    string
	InnerError error
}

func (e *BusinessError) Error() string {
	return fmt.Sprintln("ErrorCode:", e.Code, "ErrorMessage:", e.Message, "InnerError:", e.InnerError)
}

func NewBusinessError(code int, message string) *BusinessError {
	return &BusinessError{
		Code:       code,
		Message:    message,
		InnerError: nil,
	}
}

func (e *BusinessError) WithInnerError(innerError error) *BusinessError {
	e.InnerError = innerError
	return e
}

func (e *BusinessError) Panic() {
	panic(e)
}
