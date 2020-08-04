package db

import (
	"context"
	"fmt"
	"sort"
	"testing"

	"github.com/diamondburned/smolboard/smolboard"
	"github.com/go-test/deep"
	"github.com/pkg/errors"
)

func TestPostNew(t *testing.T) {
	p := NewEmptyPost("image/png")

	if name := p.Filename(); name != fmt.Sprintf("%d.png", p.ID) {
		t.Fatal("Unexpected filename:", name)
	}
}

func TestPosts(t *testing.T) {
	d := newTestDatabase(t)

	owner := testNewOwner(t, d, "ひめありかわ", "password")

	// Make 250 posts.
	var posts = make([]smolboard.Post, 250)

	t.Run("CreatePosts", func(t *testing.T) {
		tx := testBeginTx(t, d, owner.AuthToken)

		for i := 0; i < len(posts); i++ {
			p := NewEmptyPost("image/png")
			p.Size = 1
			p.Attributes = smolboard.PostAttribute{
				Width:    1920,
				Height:   1080,
				Blurhash: "klasddsjadad",
			}

			if err := tx.SavePost(&p); err != nil {
				t.Fatal("Failed to save post:", err)
			}

			posts[i] = p
		}
	})

	// Reverse all posts so that the latest ones are first.
	sort.Slice(posts, func(i, j int) bool {
		return posts[i].ID > posts[j].ID
	})

	t.Run("Paginate", func(t *testing.T) {
		tx := testBeginTx(t, d, owner.AuthToken)

		var total int

		for i := uint(0); ; i++ {
			// Ensure proper pagination. Get first page.
			p, err := tx.Posts(10, i)
			if err != nil {
				t.Fatal("Unexpected error fetching posts:", err)
			}

			if p.Total != len(posts) {
				t.Fatal("Mismatch sum count:", p.Total)
			}

			if len(p.Posts) == 0 {
				break
			}

			total += len(p.Posts)
			start := i * 10
			end := start + 10

			if eq := deep.Equal(p.Posts, posts[start:end]); eq != nil {
				t.Fatal("First page mismatch:", eq)
			}
		}

		if a, b := total, len(posts); a != b {
			t.Fatalf("Missing posts fetched from pagination: %d != %d", a, b)
		}
	})

	t.Run("PaginateInvalid", func(t *testing.T) {
		tx := testBeginTx(t, d, owner.AuthToken)

		// Limit.
		if _, err := tx.Posts(200, 0); !errors.Is(err, smolboard.ErrPageCountLimit) {
			t.Fatal("Unexpected pagination error:", err)
		}
	})

	t.Run("SetNormalPermission", func(t *testing.T) {
		// Set the first 20 posts
		tx := testBeginTx(t, d, owner.AuthToken)

		for i := 0; i < 20; i++ {
			if err := tx.SetPostPermission(posts[i].ID, smolboard.PermissionTrusted); err != nil {
				t.Fatal("Failed to set post permission:", err)
			}
		}
	})

	t.Run("PosterOverridePermission", func(t *testing.T) {
		// Create a new normal user and test.
		s := newTestUser(t, d, owner.AuthToken, "かぐやありかわ", smolboard.PermissionUser)

		tx := testBeginTx(t, d, s.AuthToken)

		// Get the first 20 posts.
		p, err := tx.Posts(20, 0)
		if err != nil {
			t.Fatal("Failed to get first 20 posts as normal user:", err)
		}

		if p.Total != 230 {
			t.Fatal("Invalid sum count != 230:", p.Total)
		}

		// Make sure that the first 20 posts are hidden properly. If this is the
		// case, then the next 20 posts should perfectly match.
		if eq := deep.Equal(p.Posts, posts[20:40]); eq != nil {
			t.Fatalf("Unexpected posts difference (len %d != 20): %q", len(p.Posts), eq)
		}

		// We don't need to test for trusted and admin, as they should work if
		// this works. This will probably be wrong and I will delete this
		// comment; but until it's gone, then that's still not the case.
	})
}

func TestNormalUploadedPosts(t *testing.T) {
	d := newTestDatabase(t)

	owner := testNewOwner(t, d, "ひめありかわ", "password")

	// Make the user account.
	s := newTestUser(t, d, owner.AuthToken, "かぐやありかわ", smolboard.PermissionUser)

	// Make 20 posts.
	var posts = make([]smolboard.Post, 20)

	t.Run("CreatePosts", func(t *testing.T) {
		tx := testBeginTx(t, d, s.AuthToken)

		for i := 0; i < len(posts); i++ {
			p := NewEmptyPost("image/png")
			p.Size = 1

			if err := tx.SavePost(&p); err != nil {
				t.Fatal("Failed to save post:", err)
			}

			posts[i] = p
		}
	})

	// Reverse all posts so that the latest ones are first.
	sort.Slice(posts, func(i, j int) bool {
		return posts[i].ID > posts[j].ID
	})

	// Apply a higher permission than the normal user.
	t.Run("ApplyHigherPermission", func(t *testing.T) {
		tx := testBeginTx(t, d, owner.AuthToken)

		for i, post := range posts {
			post.Permission = smolboard.PermissionAdministrator
			posts[i] = post

			if err := tx.SetPostPermission(post.ID, post.Permission); err != nil {
				t.Fatal("Failed to set post to admin:", err)
			}
		}
	})

	t.Run("CheckVisible", func(t *testing.T) {
		tx := testBeginTx(t, d, s.AuthToken)

		// Fetch all posts possible.
		p, err := tx.Posts(100, 0)
		if err != nil {
			t.Fatal("Failed to get all posts:", err)
		}

		if eq := deep.Equal(p.Posts, posts); eq != nil {
			t.Fatal("Posts inequality:", eq)
		}
	})
}

func TestPostNullOwner(t *testing.T) {
	d := newTestDatabase(t)

	owner := testNewOwner(t, d, "ひめありかわ", "password")
	inviteToken := testOneTimeToken(t, d, owner.AuthToken)

	var u *smolboard.Session
	err := d.AcquireGuest(context.TODO(), func(tx *Transaction) (err error) {
		u, err = tx.Signup("かぐやありかわ", "password", inviteToken, "A")
		return
	})
	if err != nil {
		t.Fatal("Failed to create normal user:", err)
	}

	p := NewEmptyPost("image/png")
	p.Size = 1

	t.Run("Setup", func(t *testing.T) {
		tx := testBeginTx(t, d, u.AuthToken)

		if err := tx.SavePost(&p); err != nil {
			t.Fatal("Failed to save post:", err)
		}

		if err := tx.DeleteUser(u.Username); err != nil {
			t.Fatal("Failed to delete self:", err)
		}
	})

	t.Run("CheckPost", func(t *testing.T) {
		tx := testBeginTx(t, d, owner.AuthToken)

		p, err := tx.Post(p.ID)
		if err != nil {
			t.Fatal("Failed to get user-deleted post:", err)
		}

		if p.Poster != nil {
			t.Fatalf("Unexpected post author: %q", *p.Poster)
		}
	})
}

func TestPostPermissions(t *testing.T) {
	for perm, test := range testPermissionSet {
		p := NewEmptyPost("image/png")
		p.Size = 1

		t.Run(perm.String(), func(t *testing.T) {
			d := newTestDatabase(t)

			owner := testNewOwner(t, d, "ひめありかわ", "password")
			ownerToken := owner.AuthToken

			auth := test.begin(t, d, owner)
			authToken := auth.AuthToken

			// Enclose this in its own test to gracefully end the transaction
			// when done.
			t.Run("PostPermission", func(t *testing.T) {
				testPostPermission(t, testBeginTx(t, d, authToken), test, &p)
			})

			// Require a separate test for this, as we need the owner's
			// transaction.
			t.Run("ReadFailPostPermission", func(t *testing.T) {
				testReadFailPermission(t, d, ownerToken, perm, test.failPerms, &p)
			})

			t.Run("DeleteSelfPost", func(t *testing.T) {
				tx := testBeginTx(t, d, authToken)

				if err := tx.DeletePost(p.ID); err != nil {
					t.Fatal("Failed to delete post:", err)
				}
			})
		})
	}
}

func testPostPermission(t *testing.T, tx *Transaction, test permTest, p *smolboard.Post) {
	t.Run("Save", func(t *testing.T) {
		if err := tx.SavePost(p); err != nil {
			t.Fatal("Failed to save post:", err)
		}
	})

	t.Run("PassPermissions", func(t *testing.T) {
		for _, perm := range test.passPerms {
			// Set the post's permission.
			if err := tx.SetPostPermission(p.ID, perm); err != nil {
				t.Fatalf("Failed to set post to permission %v: %v", perm, err)
			}

			// Try and see the post.
			_, err := tx.Post(p.ID)
			if err != nil {
				t.Fatalf("Failed to query post with permission %v: %v", perm, err)
			}
		}
	})

	t.Run("FailPermissions", func(t *testing.T) {
		for _, perm := range test.failPerms {
			// Set the permission and fail.
			err := tx.SetPostPermission(p.ID, perm)
			if !errors.Is(err, smolboard.ErrActionNotPermitted) {
				t.Fatalf("Unexpected error while trying fail perm %v: %v", perm, err)
			}
		}
	})
}

func testReadFailPermission(t *testing.T, d *Database,
	ownerToken string, targetPerm smolboard.Permission, perms []smolboard.Permission, p *smolboard.Post) {

	if len(perms) == 0 {
		return
	}

	// Register a new user.
	u := newTestUser(t, d, ownerToken, "いとうまお", targetPerm)

	for _, perm := range perms {
		t.Run("SetPermission", func(t *testing.T) {
			tx := testBeginTx(t, d, ownerToken)

			if err := tx.SetPostPermission(p.ID, perm); err != nil {
				t.Fatalf("Failed to set post permission to %v: %v", perm, err)
			}
		})

		t.Run("TryReadPost", func(t *testing.T) {
			tx := testBeginTx(t, d, u.AuthToken)

			_, err := tx.PostQuickGet(p.ID)
			if !errors.Is(err, smolboard.ErrPostNotFound) {
				t.Fatalf("Unexpected error viewing with deny permission %v: %v", perm, err)
			}

			if _, err := tx.Post(p.ID); !errors.Is(err, smolboard.ErrPostNotFound) {
				t.Fatalf("Unexpected error reading with deny permission %v: %v", perm, err)
			}
		})
	}
}

var _testPostTags = []string{
	"1boy",
	":o",
	"armband",
	"bangs",
	"blue eyes",
	"blush",
	"bow",
	"bowtie",
	"brown sweater",
	"buttons",
	"collared shirt",
	"dress shirt",
	"eyebrows visible through hair",
	"finger to cheek",
	"grey skirt",
	"hair between eyes",
	"hair ribbon",
	"long hair",
	"long sleeves",
	"looking at viewer",
	"male focus",
	"otoko no ko",
	"parted bangs",
	"pink background",
	"pink hair",
	"plaid",
	"plaid skirt",
	"pleated skirt",
	"red bow",
	"red neckwear",
	"red ribbon",
	"ribbon",
	"school uniform",
	"shiny",
	"shiny hair",
	"shirt",
	"sidelocks",
	"skirt",
	"solo",
	"sparkle background",
	"standing",
	"sweater",
	"sweater vest",
	"two side up",
	"v-neck",
	"white background",
	"white shirt",
	"wing collar",
}

func TestPostTags(t *testing.T) {
	d := newTestDatabase(t)

	owner := testNewOwner(t, d, "ひめありかわ", "password")

	tx := testBeginTx(t, d, owner.AuthToken)

	p := NewEmptyPost("image/png")
	p.Size = 1

	if err := tx.SavePost(&p); err != nil {
		t.Fatal("Failed to save post:", err)
	}

	// Insert some tags.
	var tags = _testPostTags

	// Add
	for _, tag := range tags {
		if err := tx.TagPost(p.ID, tag); err != nil {
			t.Fatalf("Failed to tag %q post: %v", tag, err)
		}
	}

	// Check exist
	q, err := tx.Post(p.ID)
	if err != nil {
		t.Fatal("Failed to query post:", err)
	}

	if eq := deep.Equal(q.Post, p); eq != nil {
		t.Fatal("Post mismatch:", eq)
	}

	var postTags = make([]string, len(q.Tags))
	for i, tag := range q.Tags {
		postTags[i] = tag.TagName
	}

	if eq := deep.Equal(postTags, tags); eq != nil {
		t.Fatal("Tags mismatch:", eq)
	}

	// Check collision
	if err := tx.TagPost(p.ID, ":o"); !errors.Is(err, smolboard.ErrTagAlreadyAdded) {
		t.Fatal("Unexpected error adding duplicate tag:", err)
	}

	// Search
	s, err := tx.SearchTag("sh")
	if err != nil {
		t.Fatal("Failed to search:", err)
	}

	if len(s) != 3 {
		t.Fatal("Invalid search length:", len(s))
	}

Search:
	for _, tag := range []string{"shiny", "shiny hair", "shirt"} {
		for _, search := range s {
			if search.TagName == tag {
				continue Search
			}
		}

		t.Errorf("Tag %q not found in search", tag)
	}

	for _, search := range s {
		if search.Count != 1 {
			t.Errorf("Searched tag %q has wrong count %d", search.TagName, search.Count)
		}
	}

	// Delete tags
	for _, old := range tags {
		if err := tx.UntagPost(p.ID, old); err != nil {
			t.Fatal("Failed to untag:", err)
		}
	}

	if err := tx.UntagPost(p.ID, "nonexistent"); !errors.Is(err, smolboard.ErrPostNotFound) {
		t.Fatal("Unexpected error untagging non-existent tag")
	}

	q, err = tx.Post(p.ID)
	if err != nil {
		t.Fatal("Failed to re-fetch post:", err)
	}

	if len(q.Tags) > 0 {
		t.Fatal("Tags found that should've been untagged:", q.Tags)
	}
}

func TestPostSearch(t *testing.T) {
	d := newTestDatabase(t)

	owner := testNewOwner(t, d, "ひめありかわ", "password")

	tx := testBeginTx(t, d, owner.AuthToken)

	p := NewEmptyPost("image/png")
	p.Size = 1

	if err := tx.SavePost(&p); err != nil {
		t.Fatal("Failed to save post:", err)
	}

	// Insert some tags.
	var tags = _testPostTags

	// Add
	for _, tag := range tags {
		if err := tx.TagPost(p.ID, tag); err != nil {
			t.Fatalf("Failed to tag %q post: %v", tag, err)
		}
	}

	sliceEq := func(t *testing.T, s smolboard.SearchResults) {
		t.Helper()

		if s.Total != 1 {
			t.Fatal("Invalid total:", s.Total)
		}

		if s.Sizes != 1 {
			t.Fatal("Invaid size:", s.Sizes)
		}

		if len(s.Posts) != 1 {
			t.Fatal("Invalid posts found with valid search query:", s)
		}

		if eq := deep.Equal(p, s.Posts[0]); eq != nil {
			t.Fatal("Returned post is different:", eq)
		}
	}

	t.Run("Author", func(t *testing.T) {
		s, err := tx.PostSearch("@ひめありかわ", 25, 0)
		if err != nil {
			t.Fatal("Failed to search:", err)
		}

		sliceEq(t, s)
	})

	t.Run("Tags", func(t *testing.T) {
		s, err := tx.PostSearch(`"otoko no ko" blush skirt`, 25, 0)
		if err != nil {
			t.Fatal("Failed to search:", err)
		}

		sliceEq(t, s)
	})

	t.Run("AuthorAndTags", func(t *testing.T) {
		s, err := tx.PostSearch(`"otoko no ko" blush @ひめありかわ skirt`, 25, 0)
		if err != nil {
			t.Fatal("Failed to search:", err)
		}

		sliceEq(t, s)
	})

	t.Run("Empty", func(t *testing.T) {
		s, err := tx.PostSearch("", 25, 0)
		if err != nil {
			t.Fatal("Failed to search:", err)
		}

		sliceEq(t, s)
	})
}
