package tabloid

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
)

type ErrorResponder interface {
	RespondError(w http.ResponseWriter, r *http.Request) bool
}

// Maybe404Error responds with not found status code, if its supplied error
// is sql.ErrNoRows.
type Maybe404Error struct {
	err error
}

func Maybe404(err error) *Maybe404Error {
	return &Maybe404Error{err: err}
}

func (e *Maybe404Error) Error() string {
	return fmt.Sprintf("Maybe404: %v", e.err.Error())
}

func (e *Maybe404Error) Is404() bool {
	return errors.Is(e.err, sql.ErrNoRows)
}

func (e *Maybe404Error) RespondError(w http.ResponseWriter, r *http.Request) bool {
	if !e.Is404() {
		return false
	}

	http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
	return true
}

// UnauthorizedError responds with unauthorized status code.
type UnauthorizedError struct {
	path string
}

func Unauthorized(path string) *UnauthorizedError {
	return &UnauthorizedError{path: path}
}

func (e *UnauthorizedError) Error() string {
	return fmt.Sprintf("UnauthorizedError: %v", e.path)
}

func (e *UnauthorizedError) RespondError(w http.ResponseWriter, r *http.Request) bool {
	http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
	return true
}

// BadRequestError responds with bad request status code
type BadRequestError struct {
	err error
}

func BadRequest(err error) *BadRequestError {
	return &BadRequestError{err: err}
}

func (e *BadRequestError) Error() string {
	return fmt.Sprintf("BadRequestError: %v", e.err)
}

func (e *BadRequestError) RespondError(w http.ResponseWriter, r *http.Request) bool {
	http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
	return true
}

// UnprocessableEntityErrorError responds with bad request status code, listing
// fields that are invalid.
type UnprocessableEntityError struct {
	fieldNames []string
	err        error
}

func UnprocessableEntity(fieldNames ...string) *UnprocessableEntityError {
	return &UnprocessableEntityError{
		fieldNames: fieldNames,
	}
}

func UnprocessableEntityWithError(err error, fieldNames ...string) *UnprocessableEntityError {
	return &UnprocessableEntityError{
		err:        err,
		fieldNames: fieldNames,
	}
}

func (e *UnprocessableEntityError) Error() string {
	if e.err != nil {
		return fmt.Sprintf("UnprocessableEntityError: error %v, %v", e.err, e.fieldNames)
	} else {
		return fmt.Sprintf("UnprocessableEntityError: %v", e.fieldNames)
	}
}

func (e *UnprocessableEntityError) RespondError(w http.ResponseWriter, r *http.Request) bool {
	msg := fmt.Sprintf("%s: invalid %v", http.StatusText(http.StatusUnprocessableEntity), e.fieldNames)
	http.Error(w, msg, http.StatusUnprocessableEntity)
	return true
}

// MethodNotAllowedError responds with a method not allowed status code.
type MethodNotAllowedError struct {
	method string
	path   string
}

func MethodNotAllowed(method string, path string) *MethodNotAllowedError {
	return &MethodNotAllowedError{
		method: method,
		path:   path,
	}
}

func (e *MethodNotAllowedError) Error() string {
	return fmt.Sprintf("MethodNotAllowed: %v %v", e.method, e.path)
}

func (e *MethodNotAllowedError) RespondError(w http.ResponseWriter, r *http.Request) bool {
	http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
	return true
}
