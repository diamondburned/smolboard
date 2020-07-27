package main

import (
	"github.com/diamondburned/smolboard/frontend/src/home"
	"github.com/diamondburned/smolboard/frontend/src/posts"
	"github.com/diamondburned/smolboard/frontend/src/postsearch"
)

func main() {
	a, err := NewApp()
	if err != nil {
		panic(err)
	}

	// Stuff that changes in the router.
	var (
		postsearch = postsearch.NewPostSearch(a)
		posts      = posts.NewPosts()
		home       = home.NewHome(postsearch)
	)

	a.AddPage("/posts", posts)
	a.AddPage("/", home)

	if err := a.Main(); err != nil {
		panic(err)
	}
}
