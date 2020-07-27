package main

import (
	"fmt"

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
}

func NewApp() (*App, error) {
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

	root := NewRoot()
	buildEnv.WireComponent(root)

	return &App{
		Root:     root,
		Router:   router,
		BuildEnv: buildEnv,
		Renderer: renderer,
	}, nil
}

func (a *App) AddRoute(route string, fn func(rm *vgrouter.RouteMatch)) {
	a.Router.MustAddRoute(route, vgrouter.RouteHandlerFunc(fn))
}

func (a *App) AddPage(route string, page vugu.Builder) {
	a.AddRoute(route, func(rm *vgrouter.RouteMatch) {
		a.SetPage(page)
	})
}

func (a *App) SetPage(page vugu.Builder) {
	a.Root.Page = page
}

func (a *App) SearchPosts(input string) {}

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
