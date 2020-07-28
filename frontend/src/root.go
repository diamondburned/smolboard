package main

//go:generate vugugen -s -r -skip-go-mod -skip-main

import (
	"github.com/diamondburned/smolboard/frontend/src/footer"
	"github.com/diamondburned/smolboard/frontend/src/home"
	"github.com/diamondburned/smolboard/frontend/src/posts"
	"github.com/diamondburned/smolboard/frontend/src/postsearch"
	"github.com/vugu/vgrouter"
	"github.com/vugu/vugu"
)

type Root struct {
	// States.
	vgrouter.NavigatorRef
	Busy bool `vugu:"data"`

	// Page and pages.
	Page  vugu.Builder
	Pages Pages

	// Components.
	PostSearch *postsearch.PostSearch
	Footer     *footer.Footer
}

type Pages struct {
	Home  *home.Home
	Posts *posts.Posts
	// TODO: error page
}

func NewRoot(p Pages) *Root {
	postSearch := postsearch.NewPostSearch()

	// p.Home.PostSearch = postSearch
	// p.Posts.PostSearch = postSearch

	return &Root{
		Page:       p.Home,
		Pages:      p,
		PostSearch: postSearch,
		Footer:     footer.NewFooter(),
	}
}

func (r *Root) MainAttrs() vugu.VGAttributeListerFunc {
	return func() []vugu.VGAttribute {
		var class = "main-container"
		if r.Busy {
			class += " loading"
		}

		return []vugu.VGAttribute{
			{
				Key: "class",
				Val: class,
			},
		}
	}
}
