package user

import (
	"net/http"
	"strconv"

	"github.com/diamondburned/smolboard/server/http/internal/form"
	"github.com/diamondburned/smolboard/server/http/internal/limit"
	"github.com/diamondburned/smolboard/server/http/internal/tx"
	"github.com/diamondburned/smolboard/server/httperr"
	"github.com/diamondburned/smolboard/smolboard"
	"github.com/go-chi/chi"
	"github.com/pkg/errors"
)

func Mount(m tx.Middlewarer) http.Handler {
	mux := chi.NewMux()
	mux.Use(limit.RateLimit(32))

	mux.Get("/", m(GetUsers))

	mux.Route("/{username}", func(r chi.Router) {
		r.Get("/", m(GetUser))
		r.Patch("/", m(PatchUser)) // only @me
		r.Delete("/", m(DeleteUser))

		r.Route("/permission", func(r chi.Router) {
			r.Patch("/", m(PromoteUser))
		})

		r.Route("/sessions", func(r chi.Router) {
			r.Get("/", m(GetSessions))
			r.Delete("/", m(DeleteAllSessions))

			// {sessionID} or @this
			r.Route(`/{sessionID:(\d+|@this)}`, func(r chi.Router) {
				r.Get("/", m(GetSession))
				r.Delete("/", m(DeleteSession))
			})
		})
	})

	return mux
}

func username(r tx.Request) string {
	var username = r.Param("username")
	if username == "@me" {
		return r.Tx.Session.Username
	}
	return username
}

func sessionID(r tx.Request) (i int64, err error) {
	if param := r.Param("sessionID"); param == "@this" {
		i = r.Tx.Session.ID
	} else {
		i, err = strconv.ParseInt(r.Param("sessionID"), 10, 64)
		if err != nil {
			return 0, errors.Wrap(err, "Failed to parse session ID")
		}
	}

	return
}

type UsersParams struct {
	Count uint `schema:"c"`
	Page  uint `schema:"p"`
}

func GetUsers(r tx.Request) (interface{}, error) {
	var p UsersParams

	if err := form.Unmarshal(r, &p); err != nil {
		return nil, errors.Wrap(err, "Invalid form")
	}

	return r.Tx.Users(p.Count, p.Page)
}

func GetUser(r tx.Request) (interface{}, error) {
	return r.Tx.User(username(r))
}

type PatchQuery struct {
	Password string `schema:"password"`
}

func PatchUser(r tx.Request) (interface{}, error) {
	// Only allow @me.
	if username(r) != r.Tx.Session.Username {
		return nil, smolboard.ErrActionNotPermitted
	}

	var q PatchQuery

	if err := form.Unmarshal(r, &q); err != nil {
		return nil, httperr.Wrap(err, 400, "Invalid form")
	}

	if q.Password != "" {
		if err := r.Tx.ChangePassword(q.Password); err != nil {
			return nil, errors.Wrap(err, "Failed to change password")
		}
	}

	u, err := r.Tx.Me()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get current user after changes")
	}

	return u, nil
}

func DeleteUser(r tx.Request) (interface{}, error) {
	return nil, r.Tx.DeleteUser(username(r))
}

func GetCurrentSession(r tx.Request) (interface{}, error) {
	return r.Tx.Session, nil
}

func GetSessions(r tx.Request) (interface{}, error) {
	return r.Tx.Sessions()
}

func GetSession(r tx.Request) (interface{}, error) {
	i, err := sessionID(r)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to parse session ID")
	}

	return r.Tx.SessionFromID(i)
}

func DeleteSession(r tx.Request) (interface{}, error) {
	i, err := sessionID(r)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to parse session ID")
	}

	return nil, r.Tx.DeleteSessionID(i)
}

func DeleteAllSessions(r tx.Request) (interface{}, error) {
	return nil, r.Tx.DeleteAllSessions()
}

type Promote struct {
	Permission smolboard.Permission `schema:"p,required"`
}

func PromoteUser(r tx.Request) (interface{}, error) {
	var p Promote

	if err := form.Unmarshal(r, &p); err != nil {
		return nil, httperr.Wrap(err, 400, "Invalid form")
	}

	return nil, r.Tx.PromoteUser(username(r), p.Permission)
}

type Authentication struct {
	Username string `schema:"username,required"`
	Password string `schema:"password,required"`
}

func Signin(r tx.Request) (interface{}, error) {
	// If already signed in.
	if !r.Tx.Session.IsZero() {
		return &r.Tx.Session, nil
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
