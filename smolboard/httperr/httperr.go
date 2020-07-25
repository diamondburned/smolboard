package httperr

import (
	"net/http"

	"github.com/pkg/errors"
)

type StatusCoder interface {
	StatusCode() int
}

func ErrCode(err error) int {
	var sc StatusCoder

	if errors.As(err, &sc) {
		return sc.StatusCode()
	}

	return 500
}

// WriteErr writes the error code.
func WriteErr(w http.ResponseWriter, err error) {
	w.WriteHeader(ErrCode(err))
}

type basicError struct {
	code int
	msg  string
}

var (
	_ error       = (*basicError)(nil)
	_ StatusCoder = (*basicError)(nil)
)

func New(code int, msg string) error {
	return basicError{code, msg}
}

func (e basicError) Error() string {
	return e.msg
}

func (e basicError) StatusCode() int {
	return e.code
}

type wrapError struct {
	code int
	wrap error
}

var (
	_ error       = (*wrapError)(nil)
	_ StatusCoder = (*wrapError)(nil)
)

func Wrap(err error, code int, msg string) error {
	if err == nil {
		return nil
	}
	return wrapError{code, errors.Wrap(err, msg)}
}

func Wrapf(err error, code int, f string, v ...interface{}) error {
	if err == nil {
		return nil
	}
	return wrapError{code, errors.Wrapf(err, f, v...)}
}

func (e wrapError) Error() string {
	return e.wrap.Error()
}

func (e wrapError) StatusCode() int {
	return e.code
}
