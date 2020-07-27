package theme

import (
	"github.com/diamondburned/smolboard/frontend/src/jsfn"
	"github.com/vugu/vugu"
)

type ThemeType uint8

const (
	LightTheme ThemeType = iota
	DarkTheme
	NordTheme
)

type Theme struct {
	Theme ThemeType `vugu:"data"`
}

func NewTheme() *Theme {
	var theme = NordTheme
	jsfn.GetLocalStorage("theme", &theme)

	return &Theme{
		Theme: theme,
	}
}

func (t *Theme) SetTheme(event vugu.DOMEvent, theme ThemeType) {
	event.StopPropagation()
	t.Theme = theme
	jsfn.SetLocalStorage("theme", t.Theme)
}

func (t *Theme) MiniCSS() (url string) {
	switch t.Theme {
	case LightTheme:
		return "https://minicss.org/flavorFiles/mini-default.min.css"
	case DarkTheme:
		return "https://minicss.org/flavorFiles/mini-dark.min.css"
	case NordTheme:
		return "https://minicss.org/flavorFiles/mini-nord.min.css"
	default:
		return ""
	}
}
