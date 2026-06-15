package runtime

import (
	"errors"
	"fmt"
)

type ErrorKind string

const (
	ErrorKindInvalidRequest ErrorKind = "invalid_request"
	ErrorKindUnauthorized   ErrorKind = "unauthorized"
	ErrorKindRateLimited    ErrorKind = "rate_limited"
	ErrorKindUnavailable    ErrorKind = "unavailable"
	ErrorKindTimeout        ErrorKind = "timeout"
	ErrorKindProvider       ErrorKind = "provider"
	ErrorKindParse          ErrorKind = "parse"
)

type Error struct {
	Kind       ErrorKind
	Engine     Engine
	StatusCode int
	Message    string
	Err        error
}

func (e *Error) Error() string {
	msg := e.Message
	if msg == "" && e.Err != nil {
		msg = e.Err.Error()
	}
	if e.StatusCode > 0 {
		return fmt.Sprintf("model runtime %s error (%s, HTTP %d): %s", e.Engine, e.Kind, e.StatusCode, msg)
	}
	return fmt.Sprintf("model runtime %s error (%s): %s", e.Engine, e.Kind, msg)
}

func (e *Error) Unwrap() error {
	return e.Err
}

func NewError(engine Engine, kind ErrorKind, status int, msg string, err error) error {
	return &Error{Kind: kind, Engine: engine, StatusCode: status, Message: msg, Err: err}
}

func Kind(err error) (ErrorKind, bool) {
	var re *Error
	if errors.As(err, &re) {
		return re.Kind, true
	}
	return "", false
}

func IsKind(err error, kind ErrorKind) bool {
	got, ok := Kind(err)
	return ok && got == kind
}

func KindFromStatus(status int) ErrorKind {
	switch status {
	case 400, 404, 422:
		return ErrorKindInvalidRequest
	case 401, 403:
		return ErrorKindUnauthorized
	case 429:
		return ErrorKindRateLimited
	default:
		if status >= 500 {
			return ErrorKindProvider
		}
		return ErrorKindUnavailable
	}
}
