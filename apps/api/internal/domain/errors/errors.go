package errors

import "errors"

var (
	ErrUnauthorized = errors.New("unauthorized")
	ErrForbidden    = errors.New("forbidden")
	ErrNotFound     = errors.New("not_found")
	ErrValidation   = errors.New("validation_error")
	ErrConflict     = errors.New("conflict")
)

type ValidationError struct {
	Message string
}

func (e ValidationError) Error() string {
	return e.Message
}

func (e ValidationError) Is(target error) bool {
	return target == ErrValidation
}

func NewValidation(message string) error {
	return ValidationError{Message: message}
}
