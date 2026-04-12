package errs

import "fmt"

type Error struct {
	Code    Code
	Message string
	Err     error
}

func (e *Error) Error() string {
	if e.Err == nil {
		return fmt.Sprintf("%s: %s", e.Code, e.Message)
	}
	return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
}

// Unwrap returns the underlying error, if any.
func (e *Error) Unwrap() error {
	return e.Err
}

// New creates a fresh error with no underlying cause.
func New(code Code, message string, args ...any) error {
	return &Error{
		Code:    code,
		Message: fmt.Sprintf(message, args...),
	}
}

// Wrap creates an error that contains another error as its cause.
func Wrap(code Code, message string, err error) error {
	return &Error{
		Code:    code,
		Message: message,
		Err:     err,
	}
}
