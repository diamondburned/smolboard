package token

import (
	"net/http"

	"github.com/diamondburned/smolboard/smolboard/http/internal/form"
	"github.com/diamondburned/smolboard/smolboard/http/internal/limit"
	"github.com/diamondburned/smolboard/smolboard/http/internal/tx"
	"github.com/diamondburned/smolboard/smolboard/httperr"
	"github.com/go-chi/chi"
)

func Mount(m tx.Middlewarer) http.Handler {
	mux := chi.NewMux()
	mux.Use(limit.RateLimit(8))
	mux.Get("/", m(ListTokens))
	mux.Post("/", m(CreateToken))
	mux.Delete("/{token}", m(DeleteToken))

	return mux
}

type TokenUses struct {
	Uses int `schema:"uses,required"`
}

func CreateToken(r tx.Request) (interface{}, error) {
	var uses TokenUses

	if err := form.Unmarshal(r, &uses); err != nil {
		return nil, httperr.Wrap(err, 400, "Invalid form")
	}

	return r.Tx.CreateToken(uses.Uses)
}

func ListTokens(r tx.Request) (interface{}, error) {
	return r.Tx.ListTokens()
}

func DeleteToken(r tx.Request) (interface{}, error) {
	return nil, r.Tx.DeleteToken(r.Param("token"))
}
