package userlist

import (
	"net/http"

	"github.com/diamondburned/smolboard/frontend/frontserver/render"
	"github.com/diamondburned/smolboard/smolboard"
	"github.com/go-chi/chi"
)

type renderCtx struct {
	render.CommonCtx
	smolboard.UserList
}

func Mount(muxer render.Muxer) http.Handler {
	mux := chi.NewMux()
	mux.Get("/", muxer.M(renderPage))
	return mux
}

func renderPage(r *render.Request) (render.Render, error) {
	panic("implement me")
}
