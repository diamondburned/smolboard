package db

import (
	"github.com/pkg/errors"
)

type Post struct {
	ID         int64      `db:"id"`
	Owner      string     `db:"owner"` // NULL!!!
	FileExt    string     `db:"fileext"`
	Permission Permission `db:"permission"`
}

// MUST TEST NULL OWNER!

type PostTag struct {
	PostID  int64  `db:"postid"`
	TagName string `db:"tagname"`
}

var ErrMissingExt = errors.New("file does not have extension")

// // SavePost saves a post to the given directory.
// func SavePost(dir string, filename string, r io.Reader, username string) (*Post, error) {

// }

// // func (d *Database)

// func (d *Database) newPost(originalName string) error {

// }
