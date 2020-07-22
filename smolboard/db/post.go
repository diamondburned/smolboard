package db

import (
	"database/sql"
	"mime"
	"strconv"

	"github.com/diamondburned/smolboard/smolboard/sniff"
	"github.com/diamondburned/smolboard/utils/httperr"
	"github.com/pkg/errors"
)

type Post struct {
	ID          int64      `db:"id"`
	Poster      string     `db:"poster"` // NULLABLE!!!
	ContentType string     `db:"contenttype"`
	Permission  Permission `db:"permission"` // TODO add checkbox for this
}

var (
	ErrMissingExt          = httperr.New(400, "file does not have extension")
	ErrPostNotFound        = httperr.New(404, "post not found")
	ErrUnsupportedFileType = httperr.New(415, "unsupported file type")
)

func NewEmptyPost(ctype string) (Post, error) {
	// Check if this is an expected content type.
	if !sniff.ContentTypeAllowed(ctype) {
		return Post{}, ErrUnsupportedFileType
	}

	return Post{
		ID:          int64(postIDGen.Generate()),
		ContentType: ctype,
		Permission:  PermissionNormal,
	}, nil
}

// Filename returns the filename that the file should be written to.
func (p Post) Filename() string {
	var sid = strconv.FormatInt(p.ID, 10)

	t, err := mime.ExtensionsByType(p.ContentType)
	if err != nil || len(t) == 0 {
		return sid
	}

	return sid + t[0]
}

func (p Post) insert(tx *sql.Tx) error {
	if p.ID == 0 || p.Poster == "" || p.ContentType == "" {
		return errors.New("cannot use empty post")
	}

	_, err := tx.Exec(
		"INSERT INTO posts VALUES (?, ?, ?, ?)",
		p.ID, p.Poster, p.ContentType, p.Permission,
	)

	if err != nil && errIsConstraint(err) {
		return ErrUserNotFound
	}

	return err
}

// MUST TEST NULL OWNER!

// TEST NON-EXISTENT USER

func (d *Transaction) Posts(count, page int) ([]Post, error) {
	// posts should do perm check, but have an explicit OR to always make their
	// images visible
	panic("Implement me")
}

func (d *Transaction) Post(id int64) (*Post, error) {
	panic("Implement me with permission check")
}

func (d *Transaction) SavePost(post *Post) error {
	// Set the post's username to the current user.
	post.Poster = d.session.Username
	// Insert into SQL with that username.
	return post.insert(d.Tx.Tx)
}

// PERMISSION CHECK TEST!

func (d *Transaction) DeletePost(id int64) error {
	// Get the post's username and evaluate if the user has permission to
	// delete.
	r := d.QueryRow("SELECT poster FROM posts WHERE id = ?", id)

	var username string
	if err := r.Scan(&username); err != nil {
		return wrapPostErr(err, "Failed to scan post's owner")
	}

	// Make sure the user performing this action is either the user being
	// deleted or an administrator.
	if err := d.HasPermOrIsUser(PermissionAdministrator, username, true); err != nil {
		return err
	}

	_, err := d.Exec("DELETE FROM posts WHERE id = ?", id)
	return wrapPostErr(err, "Failed to execute delete")
}

// SetPostPermission sets the post's permission. The current user can set the
// post's permission to as high as their own if this is their post or if the
// user is an administrator.
func (d *Transaction) SetPostPermission(id int64, target Permission) error {
	// Get the post's owner.
	var poster string

	err := d.QueryRow("SELECT poster FROM posts WHERE id = ?", id).Scan(&poster)
	if err != nil {
		return wrapPostErr(err, "Failed to scan for poster")
	}

	// This comparison is inclusive (meaning the permission can be as high as
	// the user's) if this post belongs to the user. It is NOT inclusive if this
	// post isn't the current user's.
	if err := d.HasPermission(target, poster == d.session.Username); err != nil {
		return err
	}

	_, err = d.Exec("UPDATE posts SET permission = ? WHERE id = ?", id, target)
	return wrapPostErr(err, "Failed to execute update")
}

type PostTag struct {
	PostID  int64  `db:"postid"`
	TagName string `db:"tagname"`
}

// PERMISSION CHECK IN SESSION!

func (d *Transaction) TagPost(postID int64, tag string) error {
	_, err := d.Exec("INSERT INTO posttags VALUES (?, ?)", postID, tag)
	return wrapPostErr(err, "Failed to execute insert tag")
}

func (d *Transaction) UntagPost(postID int64, tag string) error {
	_, err := d.Exec("DELETE FROM posttags VALUES (?, ?)", postID, tag)
	return wrapPostErr(err, "Failed to execute delete tag")
}

func wrapPostErr(err error, wrap string) error {
	if err != nil {
		if errIsConstraint(err) {
			return ErrPostNotFound
		}

		return errors.Wrap(err, wrap)
	}

	return nil
}
