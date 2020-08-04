// Package atomdl provides helper functions to allow atomic file downloads.
package atomdl

import (
	"io"
	"os"
	"path/filepath"

	"github.com/diamondburned/smolboard/smolboard"
	"github.com/pkg/errors"
)

func Download(r io.Reader, dir string, p *smolboard.Post) error {
	t, n, err := download(r, dir, p.Filename())
	if err != nil {
		os.Remove(t)
	}
	p.Size = n
	return err
}

func download(r io.Reader, dir, file string) (tmpname string, n int64, err error) {
	tmpname = filepath.Join(dir, "."+file)

	w, err := os.Create(tmpname)
	if err != nil {
		return tmpname, 0, errors.Wrap(err, "Failed to create file in directory")
	}
	defer w.Close()

	n, err = io.Copy(w, r)
	if err != nil {
		return tmpname, 0, errors.Wrap(err, "Failed to save uploading file")
	}

	if err := os.Rename(tmpname, filepath.Join(dir, file)); err != nil {
		return tmpname, 0, errors.Wrap(err, "Failed to move file back")
	}

	// Already moved; no need to clean up.
	return tmpname, n, nil
}
