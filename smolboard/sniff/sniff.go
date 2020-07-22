package sniff

import (
	"bufio"
	"io"
	"net/http"

	"github.com/pkg/errors"
)

// AllowedTypes contains the content types to allow.
var AllowedTypes = []string{
	// https://mimesniff.spec.whatwg.org/#matching-an-image-type-pattern
	"image/jpeg",
	"image/png",
	"image/gif",
	"image/webp",
	// https://mimesniff.spec.whatwg.org/#matching-an-audio-or-video-type-pattern
	"video/avi",
	"video/mp4",
	"video/webm",
}

func ContentTypeAllowed(ctype string) bool {
	for _, ct := range AllowedTypes {
		if ct == ctype {
			return true
		}
	}
	return false
}

// Reader ensures that Read calls will read the complete stream even after
// sniffing. Reads wrapped in Reader are inherently buffered.
type Reader struct {
	bufio.Reader
	src   io.Reader
	ctype string
}

func NewReader(r io.Reader) (*Reader, error) {
	buf := bufio.NewReaderSize(r, 512)

	h, err := buf.Peek(512)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to peek")
	}

	return &Reader{
		Reader: *buf,
		src:    r,
		ctype:  http.DetectContentType(h),
	}, nil
}

// NewLimitedReader creates a new Reader that errors out if the total read size
// is larger than given.
func NewLimitReader(r io.Reader, max int64) (*Reader, error) {
	return NewReader(newLimitedReader(r, max))
}

func (r *Reader) ContentType() string {
	return r.ctype
}

type limitedReader struct {
	io.LimitedReader
}

var ErrFileTooLarge = errors.New("file too large")

func newLimitedReader(r io.Reader, max int64) *limitedReader {
	return &limitedReader{
		io.LimitedReader{R: r, N: max + 1},
	}
}

func (r *limitedReader) Read(b []byte) (int, error) {
	n, err := r.LimitedReader.Read(b)

	if r.N <= 0 {
		return n, ErrFileTooLarge
	}

	return n, err
}
