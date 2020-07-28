package home

import (
	"github.com/vugu/vgrouter"
	"github.com/vugu/vugu"
)

type Home struct {
	vgrouter.NavigatorRef
	Input      string
	PostSearch vugu.Builder
}

func NewHome() *Home {
	return &Home{}
}
