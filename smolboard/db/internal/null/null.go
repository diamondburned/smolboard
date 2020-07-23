package null

import (
	"database/sql/driver"

	"github.com/pkg/errors"
)

var ErrUnexpectedType = errors.New("unexpected type")

// String represents a string that is empty on null.
type String string

func (s *String) Scan(v interface{}) error {
	if v == nil {
		return nil
	}

	if str, ok := v.(string); ok {
		*s = String(str)
		return nil
	}

	return errors.Wrapf(ErrUnexpectedType, "Failed to scan %#v", v)
}

func (s String) Value() (driver.Value, error) {
	if s == "" {
		return nil, nil
	}
	return string(s), nil
}

func (s String) String() string {
	return string(s)
}
