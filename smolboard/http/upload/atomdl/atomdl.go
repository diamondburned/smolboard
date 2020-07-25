// Package atomdl provides helper functions to allow atomic file downloads.
package atomdl

import (
	"io"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
)

func Download(r io.Reader, dir, file string) error {
	t, err := download(r, dir, file)
	if err != nil {
		os.Remove(t)
	}
	return err
}

func download(r io.Reader, dir, file string) (tmpname string, err error) {
	tmpname = filepath.Join(dir, "."+file)

	w, err := os.Create(tmpname)
	if err != nil {
		return tmpname, errors.Wrap(err, "Failed to create file in directory")
	}
	defer w.Close()

	if _, err := io.Copy(w, r); err != nil {
		return tmpname, errors.Wrap(err, "Failed to save uploading file")
	}

	if err := os.Rename(tmpname, filepath.Join(dir, file)); err != nil {
		return tmpname, errors.Wrap(err, "Failed to move file back")
	}

	// Already moved; no need to clean up.
	return tmpname, nil
}
