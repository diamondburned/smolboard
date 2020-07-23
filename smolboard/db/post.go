package db

import (
	"database/sql"
	"mime"
	"strconv"

	"github.com/diamondburned/smolboard/smolboard/db/internal/null"
	"github.com/diamondburned/smolboard/smolboard/sniff"
	"github.com/diamondburned/smolboard/utils/httperr"
	"github.com/pkg/errors"
)

type Post struct {
	ID          int64       `json:"id"          db:"id"`
	Poster      null.String `json:"poster"      db:"poster"`
	ContentType string      `json:"contenttype" db:"contenttype"`
	Permission  Permission  `json:"permission"  db:"permission"`
}

// PostWithTags is the type for a post with queried tags.
type PostWithTags struct {
	Post
	// Tags is manually queried externally.
	Tags []PostTag `json:"tags"`
}

var (
	ErrMissingExt          = httperr.New(400, "file does not have extension")
	ErrPostNotFound        = httperr.New(404, "post not found")
	ErrPageCountLimit      = httperr.New(400, "count is over 100 limit")
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

func (p *Post) SetPoster(poster string) {
	p.Poster = null.String(poster)
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

// Posts returns the list of posts that's paginated. Count represents the limit
// for each page and page represents the page offset 0-indexed.
func (d *Transaction) Posts(count, page uint) (posts []Post, err error) {
	p, err := d.Permission()
	if err != nil {
		return nil, err
	}

	// Limit count.
	if count > 100 {
		return nil, ErrPageCountLimit
	}

	offset := count * page

	q, err := d.Queryx(
		// This query does an explicit OR check to make sure the poster can
		// always see their posts regardless of the post's permission.
		"SELECT * FROM posts WHERE (poster = ? OR permission <= ?) ORDER BY id DESC LIMIT ?, ?",
		// SQL is dumb and wants LIMIT (offset), (count) for some reason.
		d.session.Username, p, offset, count,
	)

	if err != nil {
		return nil, errors.Wrap(err, "Failed to query for posts")
	}

	for q.Next() {
		var p Post

		if err := q.StructScan(&p); err != nil {
			return nil, errors.Wrap(err, "Failed to scan post")
		}

		posts = append(posts, p)
	}

	return
}

// Post returns a single post with the ID. It returns a post not found error if
// the post is not found or the user does not have permission to see the post.
func (d *Transaction) Post(id int64) (*PostWithTags, error) {
	p, err := d.Permission()
	if err != nil {
		return nil, err
	}

	r := d.QueryRowx(
		// Select the post only when the current user is the poster OR the
		// user's permission is less than or equal to the post's.
		"SELECT * FROM posts WHERE id = ? AND (poster = ? OR permission <= ?)",
		id, d.session.Username, p,
	)

	var post Post

	if err := r.StructScan(&post); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrPostNotFound
		}

		return nil, errors.Wrap(err, "Failed to get post")
	}

	t, err := d.Queryx("SELECT tagname FROM posttags WHERE postid = ?", id)
	if err != nil {
		// If we have no rows, then just return the post only.
		if errors.Is(err, sql.ErrNoRows) {
			return &PostWithTags{post, nil}, nil
		}

		return nil, errors.Wrap(err, "Failed to get tags")
	}

	var tags []PostTag

	// Prepared query for the sum of any tag.
	s, err := d.Prepare("SELECT COUNT(postid) FROM posttags WHERE tagname = ?")
	if err != nil {
		return nil, errors.Wrap(err, "Failed to prepare count statement")
	}

	for t.Next() {
		tag := PostTag{PostID: id}

		if err := t.Scan(&tag.TagName); err != nil {
			return nil, errors.Wrap(err, "Failed to scan tag")
		}

		if err := s.QueryRow(tag.TagName).Scan(&tag.Count); err != nil {
			return nil, errors.Wrap(err, "Failed to count tag")
		}

		tags = append(tags, tag)
	}

	return &PostWithTags{post, tags}, nil
}

func (d *Transaction) SavePost(post *Post) error {
	// Set the post's username to the current user.
	post.SetPoster(d.session.Username)
	// Insert into SQL with that username.
	return post.insert(d.Tx.Tx)
}

// canChangePost returns an error if the user cannot change this post. This
// includes deleting and tagging.
func (d *Transaction) canChangePost(postID int64) error {
	q := d.QueryRow("SELECT poster FROM posts WHERE id = ?", postID)

	var u string
	if err := q.Scan(&u); err != nil {
		return wrapPostErr(nil, err, "Failed to scan post's owner")
	}

	// Make sure the user performing this action is either the poster of the
	// post being deleted or an administrator.
	if err := d.IsUserOrHasPermOver(PermissionAdministrator, u); err != nil {
		return err
	}

	return nil
}

// PERMISSION CHECK TEST!

func (d *Transaction) DeletePost(id int64) error {
	if err := d.canChangePost(id); err != nil {
		return err
	}

	r, err := d.Exec("DELETE FROM posts WHERE id = ?", id)
	return wrapPostErr(r, err, "Failed to execute delete")
}

// SetPostPermission sets the post's permission. The current user can set the
// post's permission to as high as their own if this is their post or if the
// user is an administrator.
func (d *Transaction) SetPostPermission(id int64, target Permission) error {
	// Get the post's owner.
	var poster string

	err := d.QueryRow("SELECT poster FROM posts WHERE id = ?", id).Scan(&poster)
	if err != nil {
		return wrapPostErr(nil, err, "Failed to scan for poster")
	}

	// This comparison is inclusive (meaning the permission can be as high as
	// the user's) if this post belongs to themself. It is NOT inclusive if this
	// post isn't the current user's.
	if err := d.HasPermOverUser(target, poster); err != nil {
		return err
	}

	r, err := d.Exec("UPDATE posts SET permission = ? WHERE id = ?", target, id)
	return wrapPostErr(r, err, "Failed to execute update")
}

type PostTag struct {
	PostID  int64  `json:"post_id"  db:"postid"`
	TagName string `json:"tag_name" db:"tagname"`
	Count   int    `json:"count"    db:"-"`
}

func (d *Transaction) TagPost(postID int64, tag string) error {
	if err := d.canChangePost(postID); err != nil {
		return err
	}

	r, err := d.Exec("INSERT INTO posttags VALUES (?, ?)", postID, tag)
	return wrapPostErr(r, err, "Failed to execute insert tag")
}

func (d *Transaction) UntagPost(postID int64, tag string) error {
	if err := d.canChangePost(postID); err != nil {
		return err
	}

	r, err := d.Exec(
		"DELETE FROM posttags WHERE postid = ? AND tagname = ?",
		postID, tag,
	)

	return wrapPostErr(r, err, "Failed to execute delete tag")
}

func wrapPostErr(r sql.Result, err error, wrap string) error {
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errIsConstraint(err) {
			return ErrPostNotFound
		}

		return errors.Wrap(err, wrap)
	}

	if r != nil {
		count, err := r.RowsAffected()
		if err == nil && count == 0 {
			return ErrPostNotFound
		}
	}

	return nil
}
