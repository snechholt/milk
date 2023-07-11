package milk

import (
	"fmt"
	"net/http"
)

const StatusValidationError = 422

const (
	ErrCodeRequired     = "required"       // Value is required
	ErrCodeValueTooHigh = "value-too-high" // Numeric/date value is too high. Text field is too long.
	ErrCodeValueTooLow  = "value-too-low"  // Numeric/date value is too low. Text field is too short.
	ErrCodeDuplicate    = "duplicate"      // Duplicate. Eg username is taken or entity already exists
	ErrCodeSyntaxError  = "syntax-error"   // Invalid value. Unparseable/unreadable/not following conventions
	ErrCodeInvalidState = "invalid-state"  // Value represents/would lead to an invalid state for the object
	ErrCodeNotFound     = "not-found"      // The resource pointed by the value was not found. Eg reference to non-existing entity
)

var (
	ErrUnauthorized = NewError(http.StatusUnauthorized, "")
	ErrForbidden    = NewError(http.StatusForbidden, "")
	ErrNotFound     = NewError(http.StatusNotFound, "")
	ErrConflict     = NewError(http.StatusConflict, "")
	ErrBadRequest   = NewError(http.StatusBadRequest, "")
)

type Error struct {
	StatusCode int    `json:"statusCode"`
	Message    string `json:"message,omitempty"`
}

func NewError(statusCode int, message string) *Error {
	return &Error{
		StatusCode: statusCode,
		Message:    message,
	}
}

func (this *Error) Error() string {
	return fmt.Sprintf("API Error (%d): %s", this.StatusCode, this.Message)
}

type ValidationError struct {
	Errors []*FieldError `json:"errors,omitempty"`
}

func NewValidationError() *ValidationError {
	return &ValidationError{
		Errors: make([]*FieldError, 0),
	}
}

func (this *ValidationError) Error() string {
	return "Validation error"
}

func (this *ValidationError) AddError(key string, errorCode string) {
	this.AddErrorDetailed(key, errorCode, nil, "")
}

func (this *ValidationError) AddErrorDetailed(key string, errorCode string, data interface{}, hint string, hintArgs ...interface{}) {
	e := &FieldError{key, errorCode, fmt.Sprintf(hint, hintArgs...), data}
	this.Errors = append(this.Errors, e)
}

func (this *ValidationError) HasErrors() bool {
	return len(this.Errors) > 0
}

type FieldError struct {
	FieldName string      `json:"key"`
	ErrorCode string      `json:"errorCode"`
	Message   string      `json:"message,omitempty"`
	Data      interface{} `json:"data,omitempty"`
}
