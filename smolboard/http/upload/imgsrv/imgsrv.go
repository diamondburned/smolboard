package imgsrv

import (
	"bytes"
	"image/png"
	"net/http"
	"os"
	"path"
	"path/filepath"

	"github.com/diamondburned/smolboard/smolboard/http/internal/limit"
	"github.com/diamondburned/smolboard/smolboard/http/internal/middleware"
	"github.com/diamondburned/smolboard/smolboard/http/internal/tx"
	"github.com/diamondburned/smolboard/smolboard/httperr"
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
	mux.Route("/{file}", func(r chi.Router) {
		mux.Get("/", m(ServePost))
		// Throttle to 128 simultaneous thumbnail renders a second.
		mux.With(thumbThrottler).Get("/thumb", m(ServeThumbnail))
	})

	return mux
}

func ServePost(r tx.Request) (interface{}, error) {
	id, name := getStored(r)

	if err := r.Tx.CanViewPost(id); err != nil {
		return nil, err
	}

	return func(w http.ResponseWriter) error {
		http.ServeFile(w, r.Request, name)
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

	if err := r.Tx.CanViewPost(id); err != nil {
		return nil, err
	}

	return func(w http.ResponseWriter) error {
		t, err := imaging.FormatFromFilename(name)
		if err != nil {
			return httperr.Wrap(err, 400, "Failed to get format")
		}

		f, err := os.Open(filepath.Join(r.Up.FileDirectory, path.Base(r.URL.Path)))
		if err != nil {
			return errors.Wrap(err, "Failed to open file")
		}
		defer f.Close()

		s, err := f.Stat()
		if err != nil {
			return errors.Wrap(err, "Failed to stat file")
		}

		i, err := imaging.Decode(f, imaging.AutoOrientation(true))
		if err != nil {
			return errors.Wrap(err, "Failed to decode image")
		}

		// Early close.
		f.Close()

		var img = imaging.Fit(i, ThumbnailSize, ThumbnailSize, imaging.Lanczos)
		var buf bytes.Buffer

		if err := imaging.Encode(&buf, img, t, encopts...); err != nil {
			return errors.Wrap(err, "Failed to encode thumbnail")
		}

		http.ServeContent(w, r.Request, name, s.ModTime(), bytes.NewReader(buf.Bytes()))
		return nil
	}, nil
}
