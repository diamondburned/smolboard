package home

import "github.com/vugu/vugu"

type Home struct {
	Input      string
	PostSearch vugu.Builder
}

func NewHome(postsearch vugu.Builder) *Home {
	return &Home{
		PostSearch: postsearch,
	}
}
