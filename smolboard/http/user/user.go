package user

import (
	"net/http"

	"github.com/diamondburned/smolboard/smolboard/db"
	"github.com/diamondburned/smolboard/smolboard/http/internal/form"
	"github.com/diamondburned/smolboard/smolboard/http/internal/limit"
	"github.com/diamondburned/smolboard/smolboard/http/internal/tx"
	"github.com/diamondburned/smolboard/smolboard/httperr"
	"github.com/go-chi/chi"
	"github.com/pkg/errors"
)

func Mount(m tx.Middlewarer) http.Handler {
	mux := chi.NewMux()
	mux.Use(limit.RateLimit(32))
	mux.Route("/{username}", func(r chi.Router) {
		r.Get("/", m(GetUser))
		r.Delete("/", m(DeleteUser))

		r.Route("/permission", func(r chi.Router) {
			r.Patch("/", m(PromoteUser))
		})
	})

	return mux
}

func GetUser(r tx.Request) (interface{}, error) {
	return r.Tx.User(r.Param("username"))
}

func DeleteUser(r tx.Request) (interface{}, error) {
	return nil, r.Tx.DeleteUser(r.Param("username"))
}

type Promote struct {
	Permission db.Permission `schema:"p,required"`
}

func PromoteUser(r tx.Request) (interface{}, error) {
	var p Promote

	if err := form.Unmarshal(r, &p); err != nil {
		return nil, httperr.Wrap(err, 400, "Invalid form")
	}

	return nil, r.Tx.PromoteUser(r.Param("username"), p.Permission)
}

type Authentication struct {
	Username string `schema:"username,required"`
	Password string `schema:"password,required"`
}

func Signin(r tx.Request) (interface{}, error) {
	// If already signed in.
	if !r.Tx.Session.IsZero() {
		return nil, nil
	}

	var auth Authentication

	if err := form.Unmarshal(r, &auth); err != nil {
		return nil, httperr.Wrap(err, 400, "Invalid form")
	}

	s, err := r.Tx.Signin(auth.Username, auth.Password, r.UserAgent())
	if err != nil {
		return nil, errors.Wrap(err, "Failed to sign in")
	}

	r.SetSession(s)
	return s, nil
}

type SignupForm struct {
	Authentication
	Token string `schema:"token,required"`
}

func Signup(r tx.Request) (interface{}, error) {
	if r.Tx.Session.IsZero() {
		return nil, nil
	}

	var auth SignupForm

	if err := form.Unmarshal(r, &auth); err != nil {
		return nil, httperr.Wrap(err, 400, "Invalid form")
	}

	s, err := r.Tx.Signup(auth.Username, auth.Password, auth.Token, r.UserAgent())
	if err != nil {
		return nil, errors.Wrap(err, "Failed to sign up")
	}

	r.SetSession(s)
	return s, nil
}

func Signout(r tx.Request) (interface{}, error) {
	if r.Tx.Session.IsZero() {
		return nil, nil
	}

	if err := r.Tx.Signout(); err != nil {
		return nil, errors.Wrap(err, "Failed to sign out")
	}

	r.SetSession(nil)
	return nil, nil
}
