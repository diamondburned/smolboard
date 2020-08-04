package upload

import (
	"bufio"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"

	"github.com/bbrks/go-blurhash"
	"github.com/c2h5oh/datasize"
	"github.com/diamondburned/smolboard/server/db"
	"github.com/diamondburned/smolboard/server/http/upload/atomdl"
	"github.com/diamondburned/smolboard/server/httperr"
	"github.com/diamondburned/smolboard/smolboard"
	"github.com/disintegration/imaging"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

// BufSz is the buffer size for each upload. This is 1MB.
const BufSz = int(datasize.MB)

const MaxFiles = 128

var (
	ErrTooManyFiles = httperr.New(400, "too many files; max 128")
	ErrFileTooLarge = httperr.New(413, "file too large")
)

type ErrUnsupportedType struct {
	ContentType string
}

func (err ErrUnsupportedType) StatusCode() int {
	return 415
}

func (err ErrUnsupportedType) Error() string {
	return "unsupported file type " + err.ContentType
}

type UploadConfig struct {
	FileDirectory string            `toml:"fileDirectory"`
	MaxFileSize   datasize.ByteSize `toml:"maxFileSize"`
	AllowedTypes  []string          `toml:"allowedTypes"`
	MaxSize       MaxSize
}

func NewConfig() UploadConfig {
	return UploadConfig{
		MaxFileSize: 500 * datasize.MB,
		AllowedTypes: []string{
			"image/jpeg", "image/png", "image/gif", "image/webp",
			"video/avi", "video/mp4", "video/webm",
		},
	}
}

func (c *UploadConfig) Validate() error {
	s, err := os.Stat(c.FileDirectory)
	if err == nil {
		if !s.IsDir() {
			return fmt.Errorf("fileDirectory %q is not a directory", c.FileDirectory)
		}
	} else {
		if err := os.MkdirAll(c.FileDirectory, os.ModePerm|os.ModeDir); err != nil {
			return errors.Wrap(err, "Failed to create fileDirectory")
		}
	}

	return nil
}

func (c UploadConfig) RemovePosts(posts []*smolboard.Post) (err error) {
	for _, post := range posts {
		path := filepath.Join(c.FileDirectory, post.Filename())

		if e := os.Remove(path); e != nil {
			err = e
		}
	}
	return
}

func (c UploadConfig) CreatePosts(headers []*multipart.FileHeader) ([]*smolboard.Post, error) {
	if len(headers) > MaxFiles {
		return nil, ErrTooManyFiles
	}

	var posts = make([]*smolboard.Post, len(headers))
	var errgp = errgroup.Group{}

	for i := range headers {
		i := i

		// This creates at best 128 goroutines at once.
		errgp.Go(func() error {
			p, err := c.createPost(headers[i])
			if err != nil {
				return err
			}

			posts[i] = p
			return nil
		})
	}

	if err := errgp.Wait(); err != nil {
		return nil, err
	}

	return posts, nil
}

func (c UploadConfig) createPost(header *multipart.FileHeader) (*smolboard.Post, error) {
	// Fast path.
	if header.Size > int64(c.MaxFileSize) {
		return nil, ErrFileTooLarge
	}

	// Fast path.
	if ctype := header.Header.Get("Content-Type"); ctype != "" {
		if !c.ContentTypeAllowed(ctype) {
			return nil, ErrUnsupportedType{ctype}
		}
	}

	// Open the temporary file to read from.
	f, err := header.Open()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to open file header")
	}
	defer f.Close()

	// Wrap the temporary file reader.
	r, err := c.WrapReader(f)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create a new reader")
	}

	// Create a new empty post.
	p := db.NewEmptyPost(r.ct)

	// Download the file atomically.
	if err := atomdl.Download(r, c.FileDirectory, &p); err != nil {
		return nil, errors.Wrap(err, "Failed to download file")
	}

	var downloaded = filepath.Join(c.FileDirectory, p.Filename())

	// Blurhash time. This hash is optional, so it's fine being empty.
	i, err := imaging.Open(downloaded, imaging.AutoOrientation(true))
	if err == nil {
		bounds := i.Bounds()
		p.Attributes.Width = bounds.Dx()
		p.Attributes.Height = bounds.Dy()

		// Resize the image using a rough algorithm.
		i = imaging.Fit(i, 50, 50, imaging.Box)

		h, err := blurhash.Encode(4, 3, i)
		if err == nil {
			p.Attributes.Blurhash = h
		}
	}

	return &p, nil
}

// WrapReader wraps the given reader and restrict its MIME type as well as
// file size.
func (c UploadConfig) WrapReader(r io.Reader) (*limitedReader, error) {
	m, err := NewReader(r)
	if err != nil {
		return nil, err
	}

	if !c.ContentTypeAllowed(m.ctype) {
		return nil, ErrUnsupportedType{m.ctype}
	}

	var lim = c.MaxFileSize
	if l := c.MaxSize.SizeLimit(m.ctype); l > 0 {
		lim = l
	}

	lr := NewLimitedReader(m, int64(lim))
	lr.ct = m.ctype

	return lr, nil
}

func (c UploadConfig) ContentTypeAllowed(ctype string) bool {
	for _, ct := range c.AllowedTypes {
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

type limitedReader struct {
	rd io.LimitedReader
	wt io.WriterTo
	ct string // internal only
}

type LimitedReaderer interface {
	io.Reader
	io.WriterTo
}

var _ LimitedReaderer = (*bufio.Reader)(nil)
var _ LimitedReaderer = (*limitedReader)(nil)

func NewLimitedReader(r LimitedReaderer, max int64) *limitedReader {
	return &limitedReader{
		rd: io.LimitedReader{R: r, N: max + 1},
		wt: r,
	}
}

func (r *limitedReader) Read(b []byte) (int, error) {
	n, err := r.rd.Read(b)

	if r.rd.N <= 0 {
		return n, ErrFileTooLarge
	}

	return n, err
}

func (r *limitedReader) WriteTo(w io.Writer) (int64, error) {
	n, err := r.wt.WriteTo(w)
	r.rd.N -= n

	if r.rd.N <= 0 {
		return n, ErrFileTooLarge
	}

	return n, err
}
