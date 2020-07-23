package db

import (
	"context"
	"fmt"
	"sort"
	"testing"

	"github.com/go-test/deep"
	"github.com/pkg/errors"
)

func TestPostNew(t *testing.T) {
	p, err := NewEmptyPost("image/png")
	if err != nil {
		t.Fatal("Failed to make empty post:", err)
	}

	if name := p.Filename(); name != fmt.Sprintf("%d.png", p.ID) {
		t.Fatal("Unexpected filename:", name)
	}

	t.Run("InvalidContentType", func(t *testing.T) {
		var contentTypes = []string{
			"audio/ogg",
			"",
			"image/",
			"image/tiff",
		}

		for _, ctype := range contentTypes {
			if _, err := NewEmptyPost(ctype); !errors.Is(err, ErrUnsupportedFileType) {
				t.Fatalf("Unexpected content-type (%q) error: %v", ctype, err)
			}
		}
	})
}

func TestPosts(t *testing.T) {
	d := newTestDatabase(t)

	owner := testNewOwner(t, d, "ひめありかわ", "password")

	// Make 250 posts.
	var posts = make([]Post, 250)

	t.Run("CreatePosts", func(t *testing.T) {
		tx := testBeginTx(t, d, owner.AuthToken)

		for i := 0; i < len(posts); i++ {
			p, err := NewEmptyPost("image/png")
			if err != nil {
				t.Fatal("Failed to create post:", err)
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

			if len(p) == 0 {
				break
			}

			total += len(p)
			start := i * 10
			end := start + 10

			if eq := deep.Equal(p, posts[start:end]); eq != nil {
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
		if _, err := tx.Posts(200, 0); !errors.Is(err, ErrPageCountLimit) {
			t.Fatal("Unexpected pagination error:", err)
		}
	})

	t.Run("SetNormalPermission", func(t *testing.T) {
		// Set the first 20 posts
		tx := testBeginTx(t, d, owner.AuthToken)

		for i := 0; i < 20; i++ {
			if err := tx.SetPostPermission(posts[i].ID, PermissionTrusted); err != nil {
				t.Fatal("Failed to set post permission:", err)
			}
		}
	})

	t.Run("PosterOverridePermission", func(t *testing.T) {
		// Create a new normal user and test.
		s := newTestUser(t, d, owner.AuthToken, "かぐやありかわ", PermissionNormal)

		tx := testBeginTx(t, d, s.AuthToken)

		// Get the first 20 posts.
		p, err := tx.Posts(20, 0)
		if err != nil {
			t.Fatal("Failed to get first 20 posts as normal user:", err)
		}

		// Make sure that the first 20 posts are hidden properly. If this is the
		// case, then the next 20 posts should perfectly match.
		if eq := deep.Equal(p, posts[20:40]); eq != nil {
			t.Fatalf("Unexpected posts difference (len %d != 20): %q", len(p), eq)
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
	s := newTestUser(t, d, owner.AuthToken, "かぐやありかわ", PermissionNormal)

	// Make 20 posts.
	var posts = make([]Post, 20)

	t.Run("CreatePosts", func(t *testing.T) {
		tx := testBeginTx(t, d, s.AuthToken)

		for i := 0; i < len(posts); i++ {
			p, err := NewEmptyPost("image/png")
			if err != nil {
				t.Fatal("Failed to create post:", err)
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

	// Apply a higher permission than the normal user.
	t.Run("ApplyHigherPermission", func(t *testing.T) {
		tx := testBeginTx(t, d, owner.AuthToken)

		for i, post := range posts {
			post.Permission = PermissionAdministrator
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

		if eq := deep.Equal(p, posts); eq != nil {
			t.Fatal("Posts inequality:", eq)
		}
	})
}

func TestPostNullOwner(t *testing.T) {
	d := newTestDatabase(t)

	owner := testNewOwner(t, d, "ひめありかわ", "password")
	inviteToken := testOneTimeToken(t, d, owner.AuthToken)

	u, err := d.Signup(context.TODO(), "かぐやありかわ", "password", inviteToken, "A")
	if err != nil {
		t.Fatal("Failed to create normal user:", err)
	}

	p, err := NewEmptyPost("image/png")
	if err != nil {
		t.Fatal("Failed to create post:", err)
	}

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

		if p.Poster != "" {
			t.Fatalf("Unexpected post author: %q", p.Poster)
		}
	})
}

func TestPostPermissions(t *testing.T) {
	for perm, test := range testPermissionSet {
		p, err := NewEmptyPost("image/png")
		if err != nil {
			t.Fatal("Failed to create post:", err)
		}

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

func testPostPermission(t *testing.T, tx *Transaction, test permTest, p *Post) {
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
			if !errors.Is(err, ErrActionNotPermitted) {
				t.Fatalf("Unexpected error while trying fail perm %v: %v", perm, err)
			}
		}
	})
}

func testReadFailPermission(t *testing.T, d *Database,
	ownerToken string, targetPerm Permission, perms []Permission, p *Post) {

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

			if _, err := tx.Post(p.ID); !errors.Is(err, ErrPostNotFound) {
				t.Fatalf("Unexpected error reading with deny permission %v: %v", perm, err)
			}
		})
	}
}

func TestPostTags(t *testing.T) {
	d := newTestDatabase(t)

	owner := testNewOwner(t, d, "ひめありかわ", "password")

	tx := testBeginTx(t, d, owner.AuthToken)

	p, err := NewEmptyPost("image/png")
	if err != nil {
		t.Fatal("Failed to create post:", err)
	}

	if err := tx.SavePost(&p); err != nil {
		t.Fatal("Failed to save post:", err)
	}

	// Insert some tags.
	var tags = []string{
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

	for _, tag := range tags {
		if err := tx.TagPost(p.ID, tag); err != nil {
			t.Fatal("Failed to tag post:", err)
		}
	}

	q, err := tx.Post(p.ID)
	if err != nil {
		t.Fatal("Failed to query post:", err)
	}

	if eq := deep.Equal(q.Post, p); eq != nil {
		t.Fatal("Post mismatch:", eq)
	}

Search:
	for _, old := range tags {
		for _, tag := range q.Tags {
			if tag.TagName == old {
				if tag.Count != 1 {
					t.Errorf("Invalid tag count %d for tag %q", tag.Count, old)
				}

				if tag.PostID != q.ID {
					t.Errorf("Tag has ID mismatch: %d != %d", tag.PostID, q.ID)
				}

				continue Search
			}
		}

		t.Errorf("Tag %q not found", old)
	}

	for _, old := range tags {
		if err := tx.UntagPost(p.ID, old); err != nil {
			t.Fatal("Failed to untag:", err)
		}
	}

	if err := tx.UntagPost(p.ID, "nonexistent"); !errors.Is(err, ErrPostNotFound) {
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
