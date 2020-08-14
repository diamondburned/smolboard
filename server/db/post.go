package db

import (
	"database/sql"
	"strings"

	"github.com/diamondburned/smolboard/smolboard"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
)

func NewEmptyPost(ctype string) smolboard.Post {
	return smolboard.Post{
		ID:          int64(postIDGen.Generate()),
		ContentType: ctype,
		Permission:  smolboard.PermissionGuest,
	}
}

// PostSearch parses the query string and returns the searched posts.
func (d *Transaction) PostSearch(q string, count, page uint) (smolboard.SearchResults, error) {
	p, err := smolboard.ParsePostQuery(q)
	if err != nil {
		return smolboard.NoResults, err
	}

	return d.posts(p, count, page)
}

// Posts returns the list of posts that's paginated. Count represents the limit
// for each page and page represents the page offset 0-indexed.
func (d *Transaction) Posts(count, page uint) (smolboard.SearchResults, error) {
	return d.posts(smolboard.AllPosts, count, page)
}

func (d *Transaction) posts(pq smolboard.Query, count, page uint) (smolboard.SearchResults, error) {
	p, err := d.Permission()
	if err != nil {
		return smolboard.NoResults, err
	}

	// Limit count.
	if count > 100 {
		return smolboard.NoResults, smolboard.ErrPageCountLimit
	}

	var results = smolboard.SearchResults{
		Posts: make([]smolboard.Post, 0, count),
	}

	// Get the user if any.
	if pq.Poster != "" {
		u, err := d.User(pq.Poster)
		if err != nil {
			return smolboard.NoResults, err
		}
		results.User = u
	}

	// The worst-case benchmark showed this sqlx.In query building step to take
	// roughly 51 microseconds (us) (outdated).

	// Separate the query header to conditionally
	header := strings.Builder{}
	header.WriteString("FROM posts ")

	// This query does an explicit OR check to make sure the poster can
	// always see their posts regardless of the post's permission.
	footer := strings.Builder{}
	footer.WriteString("WHERE (posts.poster = ? OR posts.permission <= ?) ")

	// muh optimization
	footerArgs := make([]interface{}, 2, 6)
	footerArgs[0] = d.Session.Username
	footerArgs[1] = p

	if pq.Poster != "" {
		footer.WriteString("AND posts.poster = ? ")
		footerArgs = append(footerArgs, pq.Poster)
	}

	if len(pq.Tags) > 0 {
		// In order to search for tags, we'll need to join these tables.
		header.WriteString("JOIN posttags ON posttags.postid = posts.id ")
		// Query using the above joins. The HAVING COUNT query is needed to only
		// show posts with all the tags searched.
		footer.WriteString(`
			AND posttags.tagname IN (?)
			GROUP BY posts.id HAVING COUNT(posttags.tagname) = ? `)
		// There used to be a GROUP BY here. However, the GROUP BY messes up the
		// COUNT and SUM functions.
		footerArgs = append(footerArgs, pq.Tags, len(pq.Tags))
	}

	// Build the paginated query.
	query := strings.Builder{}
	query.WriteString("SELECT posts.* ")
	query.WriteString(header.String())
	query.WriteString(footer.String())
	// Sort the ID decrementally, which is latest first.
	query.WriteString("ORDER BY posts.id DESC ")

	// Append the final pagination query. SQL is dumb and wants LIMIT (offset),
	// (count) for some reason.
	query.WriteString("LIMIT ?, ?")
	queryargs := append(footerArgs, count*page, count)

	qstring, inargs, err := sqlx.In(query.String(), queryargs...)
	if err != nil {
		return smolboard.NoResults, errors.Wrap(err, "Failed to construct SQL IN query")
	}

	q, err := d.Queryx(qstring, inargs...)
	if err != nil {
		return smolboard.NoResults, errors.Wrap(err, "Failed to query for posts")
	}

	defer q.Close()

	for q.Next() {
		var p smolboard.Post

		if err := q.StructScan(&p); err != nil {
			return smolboard.NoResults, errors.Wrap(err, "Failed to scan post")
		}

		results.Posts = append(results.Posts, p)
	}

	// Save the sum count query up if there's no posts found.
	if len(results.Posts) > 0 {
		// Build the sum count query.
		countq := strings.Builder{}
		countq.WriteString(`
			SELECT
				COUNT(DISTINCT posts.id),
				SUM(posts.size) * COUNT(DISTINCT posts.id) / COUNT(posts.id) `)
		countq.WriteString(header.String())
		countq.WriteString(footer.String())

		cstring, inargs, err := sqlx.In(countq.String(), footerArgs...)
		if err != nil {
			return smolboard.NoResults, errors.Wrap(err, "Failed to construct SQL IN query")
		}

		if err := d.QueryRow(cstring, inargs...).Scan(&results.Total, &results.Sizes); err != nil {
			return smolboard.NoResults, errors.Wrap(err, "Failed to scan total posts found")
		}
	}

	return results, nil
}

// PostQuickGet gets a normal post instance. This function is used primarily
// internally, but exported for local use.
func (d *Transaction) PostQuickGet(id int64) (*smolboard.Post, error) {
	// Fast path: ignore invalid IDs.
	if id == 0 {
		return nil, smolboard.ErrPostNotFound
	}

	p, err := d.Permission()
	if err != nil {
		return nil, err
	}

	// Check if the post is there with the given constraints.
	r := d.QueryRowx(
		"SELECT * FROM posts WHERE id = ? AND (poster = ? OR permission <= ?) LIMIT 1",
		id, d.Session.Username, p,
	)

	var post smolboard.Post

	if err := r.StructScan(&post); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, smolboard.ErrPostNotFound
		}
		return nil, errors.Wrap(err, "Failed to check post")
	}

	return &post, nil
}

// Post returns a single post with the ID. It returns a post not found error if
// the post is not found or the user does not have permission to see the post.
func (d *Transaction) Post(id int64) (*smolboard.PostExtended, error) {
	// Fast path: ignore invalid IDs.
	if id == 0 {
		return nil, smolboard.ErrPostNotFound
	}

	p, err := d.Permission()
	if err != nil {
		return nil, err
	}

	r := d.QueryRowx(
		// Select the post only when the current user is the poster OR the
		// user's permission is less than or equal to the post's.
		"SELECT * FROM posts WHERE id = ? AND (poster = ? OR permission <= ?) LIMIT 1",
		id, d.Session.Username, p,
	)

	var post smolboard.Post

	if err := r.StructScan(&post); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, smolboard.ErrPostNotFound
		}

		return nil, errors.Wrap(err, "Failed to get post")
	}

	var poster *smolboard.UserPart
	if post.Poster != nil {
		poster, err = d.User(*post.Poster)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to get poster")
		}
	}

	var postEx = smolboard.PostExtended{
		Post:       post,
		PosterUser: poster,
		Tags:       []smolboard.PostTag{},
	}

	t, err := d.Queryx(`
		SELECT COUNT(1), posttags.tagname FROM posttags
		JOIN   posttags AS posttags2 ON posttags2.tagname = posttags.tagname
		WHERE  posttags2.postid = ?
		GROUP  BY posttags.tagname
		ORDER  BY posttags.tagname ASC`,
		id,
	)
	if err != nil {
		// If we have no rows, then just return the post only.
		if errors.Is(err, sql.ErrNoRows) {
			return &postEx, nil
		}

		return nil, errors.Wrap(err, "Failed to get tags")
	}

	defer t.Close()

	for t.Next() {
		tag := smolboard.PostTag{PostID: id}

		if err := t.Scan(&tag.Count, &tag.TagName); err != nil {
			return nil, errors.Wrap(err, "Failed to scan tag")
		}

		postEx.Tags = append(postEx.Tags, tag)
	}

	return &postEx, nil
}

func (d *Transaction) SavePost(post *smolboard.Post) error {
	if post.ID == 0 || post.ContentType == "" || post.Size == 0 {
		return errors.New("cannot use empty post")
	}

	if err := d.HasPermission(smolboard.PermissionUser, true); err != nil {
		return err
	}

	// Set the post's username to the current user.
	post.SetPoster(d.Session.Username)

	_, err := d.Exec(
		"INSERT INTO posts VALUES (?, ?, ?, ?, ?, ?)",
		post.ID, post.Size, post.Poster, post.ContentType, post.Permission, post.Attributes,
	)

	if err != nil && errIsConstraint(err) {
		return smolboard.ErrUserNotFound
	}

	return err
}

// canChangePost returns an error if the user cannot change this post. This
// includes deleting and tagging.
func (d *Transaction) canChangePost(postID int64) error {
	q := d.QueryRow("SELECT poster FROM posts WHERE id = ?", postID)

	var u *string
	if err := q.Scan(&u); err != nil {
		return wrapPostErr(nil, err, "Failed to scan post's owner")
	}

	var user = ""
	if u != nil {
		user = *u
	}

	// Make sure the user performing this action is either the poster of the
	// post being deleted or an administrator.
	if err := d.IsUserOrHasPermOver(smolboard.PermissionAdministrator, user); err != nil {
		return err
	}

	return nil
}

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
func (d *Transaction) SetPostPermission(id int64, target smolboard.Permission) error {
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

func validTag(tag string) error {
	return smolboard.TagIsValid(tag)
}

// SearchTag searches for tags and returns at max 25 tags. It returns only the
// count and name.
func (d *Transaction) SearchTag(part string) ([]smolboard.PostTag, error) {
	// A partial tag should still be valid.
	if err := validTag(part); err != nil {
		return nil, err
	}

	// SQL queries like these aren't the brightest idea.
	t, err := d.Queryx(`
		SELECT COUNT(1), posttags.tagname FROM posttags
		JOIN   posttags AS posttags2 ON posttags2.tagname = posttags.tagname
		WHERE  posttags2.tagname LIKE ? || '%'
		GROUP  BY posttags.tagname
		ORDER  BY COUNT(1) DESC
		LIMIT  25`,
		part,
	)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to query tags")
	}

	defer t.Close()

	var tags = []smolboard.PostTag{}

	for t.Next() {
		tag := smolboard.PostTag{}

		if err := t.Scan(&tag.Count, &tag.TagName); err != nil {
			return nil, errors.Wrap(err, "Failed to scan tag")
		}

		tags = append(tags, tag)
	}

	return tags, nil
}

func (d *Transaction) TagPost(postID int64, tag string) error {
	if err := validTag(tag); err != nil {
		return err
	}

	if err := d.canChangePost(postID); err != nil {
		return err
	}

	r, err := d.Exec("INSERT INTO posttags VALUES (?, ?)", postID, tag)
	if err != nil {
		if errIsConstraint(err) {
			return smolboard.ErrTagAlreadyAdded
		}
	}
	return wrapPostErr(r, err, "Failed to execute insert tag")
}

func (d *Transaction) UntagPost(postID int64, tag string) error {
	if err := validTag(tag); err != nil {
		return err
	}

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
			return smolboard.ErrPostNotFound
		}

		return errors.Wrap(err, wrap)
	}

	if r != nil {
		count, err := r.RowsAffected()
		if err == nil && count == 0 {
			return smolboard.ErrPostNotFound
		}
	}

	return nil
}
