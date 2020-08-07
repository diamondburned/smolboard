package limread

import (
	"bufio"
	"fmt"
	"io"
	"net/http"

	"github.com/c2h5oh/datasize"
	"github.com/diamondburned/smolboard/server/http/internal/middleware"
	"github.com/pkg/errors"
)

// BufSz is the buffer size for each upload. This is 1MB.
const BufSz = int(256 * datasize.KB)

type ErrFileTooLarge struct {
	Max   int64
	CType string
}

func (err ErrFileTooLarge) StatusCode() int {
	return 413
}

func (err ErrFileTooLarge) Error() string {
	var str = fmt.Sprintf(
		"File too large, maximum size allowed is %s",
		datasize.ByteSize(err.Max).HumanReadable(),
	)

	if err.CType != "" {
		str += " for type " + err.CType
	}

	return str
}

func LimitBody(size datasize.ByteSize) middleware.F {
	return middleware.P(func(w http.ResponseWriter, r *http.Request) bool {
		r.Body = newReadCloser(
			NewLimitedReader(
				bufio.NewReader(r.Body),
				int64(size.Bytes()),
			),
			r.Body,
		)
		return true
	})
}

type readCloser struct {
	io.Reader
	io.Closer
}

func newReadCloser(r io.Reader, c io.Closer) readCloser {
	return readCloser{r, c}
}

// Reader ensures that Read calls will read the complete stream even after
// sniffing. Reads wrapped in Reader are inherently buffered.
type Reader struct {
	bufio.Reader
	src   io.Reader
	ctype string
}

func NewReader(r io.Reader) (*Reader, error) {
	buf := bufio.NewReaderSize(r, BufSz)

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

func (r *Reader) ContentType() string {
	return r.ctype
}

type LimitedReader struct {
	reader io.LimitedReader
	writer io.WriterTo
	Bytes  int64
	CType  string // internal only
}

type LimitedReaderer interface {
	io.Reader
	io.WriterTo
}

var (
	_ LimitedReaderer = (*bufio.Reader)(nil)
	_ LimitedReaderer = (*LimitedReader)(nil)
)

func NewLimitedReader(r LimitedReaderer, max int64) *LimitedReader {
	return &LimitedReader{
		reader: io.LimitedReader{R: r, N: max + 1},
		writer: r,
		Bytes:  max,
	}
}

func (r *LimitedReader) Read(b []byte) (int, error) {
	n, err := r.reader.Read(b)

	if r.reader.N <= 0 {
		return n, ErrFileTooLarge{Max: r.Bytes, CType: r.CType}
	}

	return n, err
}

func (r *LimitedReader) WriteTo(w io.Writer) (int64, error) {
	n, err := r.writer.WriteTo(w)
	r.reader.N -= n

	if r.reader.N <= 0 {
		return n, ErrFileTooLarge{Max: r.Bytes, CType: r.CType}
	}

	return n, err
}
