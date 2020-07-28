package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/diamondburned/smolboard/client"
	"github.com/diamondburned/smolboard/frontend/src/home"
	"github.com/diamondburned/smolboard/frontend/src/posts"
	"github.com/pkg/errors"
	"github.com/vugu/vgrouter"
	"github.com/vugu/vugu"
	"github.com/vugu/vugu/domrender"
)

type App struct {
	*Root
	Router   *vgrouter.Router
	BuildEnv *vugu.BuildEnv
	Renderer *domrender.JSRenderer

	Session *client.Session

	// vugu is trash so we need this
	draw chan struct{}
}

func NewApp(s *client.Session) (*App, error) {
	buildEnv, err := vugu.NewBuildEnv()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create build env")
	}

	renderer, err := domrender.New("#app")
	if err != nil {
		return nil, errors.Wrap(err, "Failed to grab DOM element #app")
	}

	router := vgrouter.New(renderer.EventEnv())
	router.SetNotFound(vgrouter.RouteHandlerFunc(
		func(rm *vgrouter.RouteMatch) {
			fmt.Println("Route not fuond.")
		},
	))

	buildEnv.SetWireFunc(func(b vugu.Builder) {
		if naver, ok := b.(vgrouter.NavigatorSetter); ok {
			naver.NavigatorSet(router)
		}
	})

	app := &App{
		Router:   router,
		BuildEnv: buildEnv,
		Renderer: renderer,
		Session:  s,
	}

	// Stuff that changes in the router.
	app.Root = NewRoot(Pages{
		home.NewHome(),
		posts.NewPosts(s),
	})
	buildEnv.WireComponent(app.Root)

	router.MustAddRoute("/posts", vgrouter.RouteHandlerFunc(app.posts))
	router.MustAddRouteExact("/", vgrouter.RouteHandlerFunc(
		func(rm *vgrouter.RouteMatch) { app.Root.Page = app.Pages.Home },
	))

	return app, nil
}

func (a *App) getLock() vugu.EventEnv {
	return a.Renderer.EventEnv()
}

func (a *App) update(fn func()) {
	env := a.getLock()
	env.Lock()
	fn()
	env.UnlockRender()
}

func (a *App) posts(rm *vgrouter.RouteMatch) {
	// If exact: /posts?... => show gallery
	if rm.Exact {
		a.Root.Page = a.Pages.Posts
		a.Root.Busy = true

		q := rm.Params.Get("q")
		p, _ := strconv.Atoi(rm.Params.Get("p"))

		go func() {
			p, err := a.Session.PostSearch(q, 25, p)
			if err != nil {
				fmt.Println("Error searching post:", err)
			}

			time.Sleep(2 * time.Second)

			a.update(func() {
				a.Root.Busy = false
				a.Pages.Posts.SetResults(p)
			})
		}()

	} else {
		// Grab the post ID.
		idstring := strings.TrimPrefix(rm.Path, "/posts/")
		id, err := strconv.Atoi(idstring)
		if err != nil {
			fmt.Println("Invalid ID:", err)
		}

		fmt.Println("Got ID:", id)
	}
}

// Main starts the event loop.
func (a *App) Main() error {
	if err := a.Router.ListenForPopState(); err != nil {
		return errors.Wrap(err, "Failed to listen for pop state")
	}

	if err := a.Router.Pull(); err != nil {
		return errors.Wrap(err, "Failed to pull router")
	}

	defer a.Renderer.Release()

	// Force the render loop to run at least once.
	for ok := true; ok; ok = a.Renderer.EventWait() {
		if err := a.Renderer.Render(a.BuildEnv.RunBuild(a)); err != nil {
			return err
		}
	}

	return nil
}
