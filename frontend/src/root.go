package main

//go:generate vugugen -s -r -skip-go-mod -skip-main

import (
	"github.com/diamondburned/smolboard/frontend/src/footer"
	"github.com/vugu/vgrouter"
	"github.com/vugu/vugu"
)

type Root struct {
	vgrouter.NavigatorRef
	Page vugu.Builder

	Footer *footer.Footer
}

func NewRoot() *Root {
	return &Root{
		Footer: footer.NewFooter(),
	}
}
