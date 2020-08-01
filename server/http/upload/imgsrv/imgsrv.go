package imgsrv

import (
	"bytes"
	"image/png"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"

	"github.com/diamondburned/smolboard/server/http/internal/limit"
	"github.com/diamondburned/smolboard/server/http/internal/middleware"
	"github.com/diamondburned/smolboard/server/http/internal/tx"
	"github.com/diamondburned/smolboard/server/httperr"
	"github.com/disintegration/imaging"
	"github.com/go-chi/chi"
	"github.com/pkg/errors"
)

// ThumbnailSize controls the dimension of the thumbnail.
const ThumbnailSize = 256

var (
	ErrFileNotFound = httperr.New(404, "file not found")
)

var thumbThrottler = middleware.Throttle(128)

func Mount(m tx.Middlewarer) http.Handler {
	mux := chi.NewMux()
	mux.Use(limit.RateLimit(128)) // 128 accesses per second

	// Parse the filename for the post ID.
	mux.With(parseID).Route("/{file}", func(r chi.Router) {
		r.Get("/", m(ServePost))
		// Throttle to 128 simultaneous thumbnail renders a second.
		r.With(thumbThrottler).Get("/thumb", m(ServeThumbnail))
	})

	return mux
}

func ServePost(r tx.Request) (interface{}, error) {
	id, name := getStored(r)
	log.Printf("id=%d,name=%q\n", id, name)

	p, err := r.Tx.PostQuickGet(id)
	if err != nil {
		return nil, err
	}

	return func(w http.ResponseWriter) error {
		// Set up caching. Max age is 7 days.
		w.Header().Set("Cache-Control", "private, max-age=604800")

		// If the user requested post's extension is different from what we
		// have, then we do a permanent redirection to the correct filename.
		if filename := p.Filename(); filename != name {
			redirect := path.Dir(r.URL.Path) + "/" + filename
			// Cache the redirect for this specific endpoint.
			http.Redirect(w, r.Request, redirect, http.StatusPermanentRedirect)

			return nil
		}

		var filepath = filepath.Join(r.Up.FileDirectory, name)

		// Try and stat the file for the modTime to be used as the ETag. If we
		// can't stat the file, then don't serve anything.
		s, err := os.Stat(filepath)
		if err != nil {
			return errors.Wrap(err, "Failed to stat file")
		}

		// Write the ETag as a Unix timestamp in nanoseconds hexadecimal.
		w.Header().Set("ETag", strconv.FormatInt(s.ModTime().UnixNano(), 16))

		// ServeFile will actually validate the ETag for us.
		http.ServeFile(w, r.Request, filepath)

		return nil
	}, nil
}

var encopts = []imaging.EncodeOption{
	imaging.JPEGQuality(98),
	// HTTP already compresses, so we save CPU.
	imaging.PNGCompressionLevel(png.DefaultCompression),
}

func ServeThumbnail(r tx.Request) (interface{}, error) {
	id, name := getStored(r)
	log.Printf("id=%d,name=%q\n", id, name)

	p, err := r.Tx.PostQuickGet(id)
	if err != nil {
		return nil, err
	}

	return func(w http.ResponseWriter) error {
		// Try serving the thumbnail and redirect the user to the original
		// content if there's none available.
		if !serveThumbnail(w, r, p.Filename()) {
			redirect := path.Dir(r.URL.Path) // remove /thumb
			http.Redirect(w, r.Request, redirect, http.StatusPermanentRedirect)
		}

		// This never fails.
		return nil
	}, nil
}

func serveThumbnail(w http.ResponseWriter, r tx.Request, name string) bool {
	t, err := imaging.FormatFromFilename(name)
	if err != nil {
		return false
	}

	f, err := os.Open(filepath.Join(r.Up.FileDirectory, name))
	if err != nil {
		return false
	}
	defer f.Close()

	s, err := f.Stat()
	if err != nil {
		return false
	}

	i, err := imaging.Decode(f, imaging.AutoOrientation(true))
	if err != nil {
		return false
	}

	// Early close.
	f.Close()

	var img = imaging.Fit(i, ThumbnailSize, ThumbnailSize, imaging.Lanczos)
	var buf bytes.Buffer

	if err := imaging.Encode(&buf, img, t, encopts...); err != nil {
		return false
	}

	// Before serving the content, we could use the ModTime as the ETag for
	// caching validation.
	var modTime = s.ModTime()
	w.Header().Set("ETag", strconv.FormatInt(modTime.UnixNano(), 16))

	// Set up caching. Max age is 7 days.
	w.Header().Set("Cache-Control", "private, max-age=604800")

	http.ServeContent(w, r.Request, name, modTime, bytes.NewReader(buf.Bytes()))
	return true
}
