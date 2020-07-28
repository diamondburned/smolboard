package postsearch

import (
	"net/url"

	"github.com/vugu/vgrouter"
)

type PostSearch struct {
	vgrouter.NavigatorRef
}

func NewPostSearch() *PostSearch {
	return &PostSearch{}
}

func (ps *PostSearch) Search(input string) {
	ps.Navigate("/posts", url.Values{
		"q": {input},
	})
}
