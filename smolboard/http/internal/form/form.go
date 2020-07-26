package form

import (
	"net/http"

	"github.com/c2h5oh/datasize"
	"github.com/diamondburned/smolboard/smolboard/http/internal/tx"
	"github.com/gorilla/schema"
	"github.com/pkg/errors"
)

const MaxMemory = int64(2 * datasize.MB)

var decoder = schema.NewDecoder()

// Unmarshal decodes the form in the given request into the interface.
func Unmarshal(r tx.Request, v interface{}) error {
	if err := r.ParseMultipartForm(MaxMemory); err != nil {
		return errors.Wrap(err, "Failed to parse form")
	}

	// Prioritize multipart.
	if r.MultipartForm != nil {
		return decoder.Decode(v, r.MultipartForm.Value)
	}

	switch r.Method {
	case http.MethodPatch, http.MethodPost, http.MethodPut:
		return decoder.Decode(v, r.PostForm)
	default:
		return decoder.Decode(v, r.Form)
	}
}
