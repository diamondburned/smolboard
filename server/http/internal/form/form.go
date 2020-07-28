package form

import (
	"net/http"

	"github.com/c2h5oh/datasize"
	"github.com/diamondburned/smolboard/server/http/internal/tx"
	"github.com/gorilla/schema"
	"github.com/pkg/errors"
)

const MaxMemory = int64(2 * datasize.MB)

var decoder = schema.NewDecoder()

// Unmarshal decodes the form in the given request into the interface.
func Unmarshal(r tx.Request, v interface{}) error {
	// Prioritize multipart.
	if err := r.ParseMultipartForm(MaxMemory); err == nil && r.MultipartForm != nil {
		return decoder.Decode(v, r.MultipartForm.Value)
	}

	if err := r.ParseForm(); err != nil {
		return errors.Wrap(err, "Failed to parse form")
	}

	switch r.Method {
	case http.MethodPatch, http.MethodPost, http.MethodPut:
		return decoder.Decode(v, r.PostForm)
	default:
		return decoder.Decode(v, r.Form)
	}
}
