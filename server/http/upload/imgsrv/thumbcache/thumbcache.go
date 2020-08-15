package thumbcache

import (
	"io"
	"os"
	"path/filepath"

	"github.com/c2h5oh/datasize"
	"github.com/peterbourgon/diskv"
)

var thumbCache = diskv.New(diskv.Options{
	BasePath: filepath.Join(os.TempDir(), "smolboard-thumbs"),
	Transform: func(s string) []string {
		return nil
	},
	// 4MB cache in memory strictly.
	CacheSizeMax: uint64(4 * datasize.MB),
})

func Get(name string) ([]byte, error) {
	b, err := thumbCache.Read(name)
	if err != nil {
		return nil, err
	}

	if len(b) == 0 {
		return nil, io.ErrUnexpectedEOF
	}

	return b, nil
}

func Put(name string, b []byte) error {
	return thumbCache.Write(name, b)
}

func Delete(name string) error {
	return thumbCache.Erase(name)
}
