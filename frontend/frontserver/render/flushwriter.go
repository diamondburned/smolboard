package render

import (
	"net/http"
)

type FlushWriter interface {
	http.ResponseWriter
	http.Flusher
}

func TryFlushWriter(w http.ResponseWriter) FlushWriter {
	return newFlusher(w)
}

type flusher struct {
	http.ResponseWriter
	f http.Flusher
}

func newFlusher(w http.ResponseWriter) FlushWriter {
	var f = flusher{ResponseWriter: w}
	if flusher, ok := w.(http.Flusher); ok {
		f.f = flusher
	}
	return f
}

func (f flusher) Write(b []byte) (int, error) {
	n, err := f.ResponseWriter.Write(b)
	if err == nil {
		f.Flush()
	}
	return n, err
}

func (f flusher) Flush() {
	if f.f != nil {
		f.f.Flush()
	}
}
