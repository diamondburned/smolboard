package thumbcache

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/c2h5oh/datasize"
	"github.com/peterbourgon/diskv"
)

var thumbCache = diskv.New(diskv.Options{
	BasePath: filepath.Join(os.TempDir(), "smolboard-thumbs"),
	Transform: func(s string) []string {
		return []string{url.PathEscape(strings.TrimRight(s, "/"))}
	},
	// 4MB cache in memory strictly.
	CacheSizeMax: uint64(4 * datasize.MB),
})

func Get(name string) ([]byte, error) {
	return thumbCache.Read(name)
}

func Put(name string, b []byte) error {
	return thumbCache.Write(name, b)
}

func Delete(name string) error {
	return thumbCache.Erase(name)
}
