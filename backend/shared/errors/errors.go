package errs

import (
	"errors"
	"fmt"
	"maps"
)

type Error struct {
	Code    Code
	Message string
	Err     error

	// Status is the HTTP status of the call that failed, when the error came
	// from one (e.g. an upstream API answering 401 or 429). 0 when unknown.
	Status int
	// Details holds structured context for logs and callers, e.g. the model or
	// the missing config. Not part of Error().
	Details map[string]any
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

// Option sets an optional field on an Error.
type Option func(*Error)

// WithStatus records the HTTP status of the failed call.
func WithStatus(status int) Option {
	return func(e *Error) {
		e.Status = status
	}
}

// WithDetail adds one key to Details.
func WithDetail(key string, value any) Option {
	return func(e *Error) {
		if e.Details == nil {
			e.Details = map[string]any{}
		}
		e.Details[key] = value
	}
}

// WithDetails merges details into Details.
func WithDetails(details map[string]any) Option {
	return func(e *Error) {
		if e.Details == nil {
			e.Details = make(map[string]any, len(details))
		}
		maps.Copy(e.Details, details)
	}
}

// New creates a fresh error with no underlying cause.
func New(code Code, message string, args ...any) error {
	return &Error{
		Code:    code,
		Message: fmt.Sprintf(message, args...),
	}
}

// NewWith is New with options instead of format arguments.
func NewWith(code Code, message string, opts ...Option) error {
	e := &Error{Code: code, Message: message}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// Wrap creates an error that contains another error as its cause.
func Wrap(code Code, message string, err error, opts ...Option) error {
	e := &Error{
		Code:    code,
		Message: message,
		Err:     err,
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// StatusOf returns the first non-zero Status in err's chain, or 0. A wrapper
// without a status of its own reports its cause's, so an upstream 429 is still
// visible after the adapter wraps it.
func StatusOf(err error) int {
	for err != nil {
		var e *Error
		if !errors.As(err, &e) {
			return 0
		}
		if e.Status != 0 {
			return e.Status
		}
		err = e.Err
	}
	return 0
}

// HasCode reports whether any *Error in err's chain has code, so a caller can
// branch on a code without caring which layer wrapped it.
func HasCode(err error, code Code) bool {
	for err != nil {
		var e *Error
		if !errors.As(err, &e) {
			return false
		}
		if e.Code == code {
			return true
		}
		err = e.Err
	}
	return false
}

// DetailsOf merges Details along err's chain; outer errors win on a shared key.
func DetailsOf(err error) map[string]any {
	var out map[string]any
	for err != nil {
		var e *Error
		if !errors.As(err, &e) {
			break
		}
		for k, v := range e.Details {
			if out == nil {
				out = map[string]any{}
			}
			if _, ok := out[k]; !ok {
				out[k] = v
			}
		}
		err = e.Err
	}
	return out
}
