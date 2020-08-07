package upload

import (
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"os"
	"path/filepath"

	"github.com/bbrks/go-blurhash"
	"github.com/c2h5oh/datasize"
	"github.com/diamondburned/smolboard/server/db"
	"github.com/diamondburned/smolboard/server/http/internal/limread"
	"github.com/diamondburned/smolboard/server/http/upload/atomdl"
	"github.com/diamondburned/smolboard/server/http/upload/ff"
	"github.com/diamondburned/smolboard/server/http/upload/imgsrv/thumbcache"
	"github.com/diamondburned/smolboard/server/httperr"
	"github.com/diamondburned/smolboard/smolboard"
	"github.com/disintegration/imaging"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

const MaxFiles = 128

var ErrTooManyFiles = httperr.New(400, "too many files; max 128")

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

// CleanupPost cleans up a single post asynchronously.
func (c UploadConfig) CleanupPost(post smolboard.Post) {
	c.CleanupPosts([]*smolboard.Post{&post})
}

// CleanupPosts cleans up posts asynchronously.
func (c UploadConfig) CleanupPosts(posts []*smolboard.Post) {
	go func() {
		for _, post := range posts {
			if post != nil {
				fil := post.Filename()
				err := os.Remove(filepath.Join(c.FileDirectory, fil))
				// Log the error if we have one and it's not a "file not found"
				// error.
				if err != nil && !errors.Is(err, os.ErrNotExist) {
					log.Printf("Failed to cleanup %q: %v", fil, err)
				}

				// Make sure the thumbnail is not cached anymore.
				thumbcache.Delete(fil)
			}
		}
	}()
}

func (c UploadConfig) CreatePosts(headers []*multipart.FileHeader) ([]*smolboard.Post, error) {
	if len(headers) > MaxFiles {
		return nil, ErrTooManyFiles
	}

	// Fast path: check all incoming files before starting goroutines to
	// asynchronously download them.
	for _, header := range headers {
		// Fast path.
		if header.Size > int64(c.MaxFileSize) {
			return nil, limread.ErrFileTooLarge{Max: int64(c.MaxFileSize)}
		}

		// Fast path.
		if ctype := header.Header.Get("Content-Type"); ctype != "" {
			if !c.ContentTypeAllowed(ctype) {
				return nil, ErrUnsupportedType{ctype}
			}
		}
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
		// Clean up all downloaded files on error.
		c.CleanupPosts(posts)

		return nil, err
	}

	return posts, nil
}

func (c UploadConfig) createPost(header *multipart.FileHeader) (*smolboard.Post, error) {
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
	p := db.NewEmptyPost(r.CType)

	// Download the file atomically.
	if err := atomdl.Download(r, c.FileDirectory, &p); err != nil {
		return nil, errors.Wrap(err, "Failed to save file")
	}

	var downloaded = filepath.Join(c.FileDirectory, p.Filename())

	// Try parsing the file as an image.
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
	} else {
		// Failed to parse above as a normal image. Resort to shelling out, if
		// possible.
		s, err := ff.ProbeSize(downloaded)
		if err == nil {
			p.Attributes.Width = s.Width
			p.Attributes.Height = s.Height
		}

		i, err := ff.FirstFrame(downloaded, 50, 50, ff.NeighborScaler)
		if err == nil {
			h, err := blurhash.Encode(4, 3, i)
			if err == nil {
				p.Attributes.Blurhash = h
			}
		}
	}

	return &p, nil
}

// WrapReader wraps the given reader and restrict its MIME type as well as
// file size.
func (c UploadConfig) WrapReader(r io.Reader) (*limread.LimitedReader, error) {
	m, err := limread.NewReader(r)
	if err != nil {
		return nil, err
	}

	if !c.ContentTypeAllowed(m.ContentType()) {
		return nil, ErrUnsupportedType{m.ContentType()}
	}

	var lim = c.MaxFileSize
	if l := c.MaxSize.SizeLimit(m.ContentType()); l > 0 {
		lim = l
	}

	lr := limread.NewLimitedReader(m, int64(lim))
	lr.CType = m.ContentType()

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
