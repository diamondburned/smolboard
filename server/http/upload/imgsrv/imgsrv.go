package imgsrv

import (
	"bytes"
	"image"
	"image/draw"
	"image/jpeg"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"

	"github.com/diamondburned/smolboard/server/http/internal/limit"
	"github.com/diamondburned/smolboard/server/http/internal/tx"
	"github.com/diamondburned/smolboard/server/http/upload/ff"
	"github.com/diamondburned/smolboard/server/http/upload/imgsrv/thumbcache"
	"github.com/diamondburned/smolboard/server/httperr"
	"github.com/disintegration/imaging"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/pkg/errors"
)

// ThumbnailSize controls the dimension of the thumbnail.
const ThumbnailSize = 400

var (
	ErrFileNotFound = httperr.New(404, "file not found")
)

// limit the thumbnail processors to 50 simultaneous requests.
var thumbThrottler = middleware.Throttle(50)

func Mount(m tx.Middlewarer) http.Handler {
	mux := chi.NewMux()
	mux.Use(limit.RateLimit(100)) // 100 accesses per second

	// Parse the filename for the post ID.
	mux.With(parseID).Route("/{file}", func(r chi.Router) {
		r.Get("/", m(ServePost))

		// Throttle to 128 simultaneous thumbnail renders a second.
		r.With(thumbThrottler).Group(func(r chi.Router) {
			r.Get("/thumb.jpeg", m(ServeThumbnail))
			r.Get("/thumb.jpg", m(ServeThumbnail))
		})
	})

	return mux
}

func ServePost(r tx.Request) (interface{}, error) {
	id, name := getStored(r)

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

func ServeThumbnail(r tx.Request) (interface{}, error) {
	id, _ := getStored(r)

	p, err := r.Tx.PostQuickGet(id)
	if err != nil {
		return nil, err
	}

	return func(w http.ResponseWriter) error {
		var name = p.Filename()

		// Try serving the thumbnail and redirect the user to the original
		// content if there's none available.
		if err := serveThumbnail(w, r, name); err != nil {
			log.Printf("Error serving thumbnail %q: %v\n", name, err)

			redirect := path.Dir(r.URL.Path) // remove /thumb
			http.Redirect(w, r.Request, redirect, http.StatusPermanentRedirect)
		}

		// This never fails.
		return nil
	}, nil
}

var jpegOpts = &jpeg.Options{
	Quality: 95,
}

func serveThumbnail(w http.ResponseWriter, r tx.Request, name string) error {
	var path = filepath.Join(r.Up.FileDirectory, name)

	// We should always check if the file still exists. It may not.
	s, err := os.Stat(path)
	if err != nil {
		// Cleanup if any. This isn't important, so we can ignore.
		thumbcache.Delete(name)

		return errors.Wrap(err, "Failed to stat file")
	}

	var modTime = s.ModTime()

	// Before serving the content, we could use the ModTime as the ETag for
	// caching validation.
	w.Header().Set("ETag", strconv.FormatInt(modTime.UnixNano(), 16))

	// Set up caching. Max age is 7 days.
	w.Header().Set("Cache-Control", "private, max-age=604800")

	// Check if the file is in the cache. If it is, return.
	b, err := thumbcache.Get(name)
	if err == nil {
		http.ServeContent(w, r.Request, "thumb.jpeg", modTime, bytes.NewReader(b))
		return nil
	}

	b, err = tryNativeJPEG(r, path)
	if err != nil {
		b, err = tryFFmpeg(r, path)
	}

	if err != nil {
		return err
	}

	// Non-fatal cache error; ignore.
	if err := thumbcache.Put(name, b); err != nil {
		log.Println("Failed to cache thumbnail:", err)
	}

	http.ServeContent(w, r.Request, "thumb.jpeg", modTime, bytes.NewReader(b))
	return nil
}

func tryFFmpeg(r tx.Request, path string) ([]byte, error) {
	return ff.FirstFrameJPEG(path, ThumbnailSize, ThumbnailSize, ff.LanczosScaler)
}

func tryNativeJPEG(r tx.Request, path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to open file")
	}
	defer f.Close()

	i, err := imaging.Decode(f, imaging.AutoOrientation(true))
	if err != nil {
		return nil, errors.Wrap(err, "Failed to decode image")
	}

	// Early close.
	f.Close()

	nrgba := imaging.Fit(i, ThumbnailSize, ThumbnailSize, imaging.Lanczos)
	// Since JPEG really wants an *RGBA, we need to redraw everything.
	rgba := image.NewRGBA(nrgba.Rect)
	draw.Draw(rgba, rgba.Rect, nrgba, rgba.Rect.Min, draw.Src)

	var buf bytes.Buffer

	if err := jpeg.Encode(&buf, rgba, jpegOpts); err != nil {
		return nil, errors.Wrap(err, "Failed to encode JPEG")
	}

	return buf.Bytes(), nil
}
