package render

import (
	"context"
	"net/http"
)

type Theme uint8

const (
	LightTheme Theme = iota
	DarkTheme
	NordTheme // also light

	// reserved for internal use
	themeLen
)

const DefaultTheme = LightTheme

func ParseTheme(name string) Theme {
	switch name {
	case "light":
		return LightTheme
	case "dark":
		return DarkTheme
	case "nord":
		return NordTheme
	}

	return DefaultTheme
}

func (t Theme) String() string {
	switch t {
	case LightTheme:
		return "light"
	case DarkTheme:
		return "dark"
	case NordTheme:
		fallthrough
	default:
		return "nord"
	}
}

func (t Theme) URL() string {
	switch t {
	case LightTheme:
		return "https://cdnjs.cloudflare.com/ajax/libs/mini.css/3.0.1/mini-default.min.css"
	case DarkTheme:
		return "https://cdnjs.cloudflare.com/ajax/libs/mini.css/3.0.1/mini-dark.min.css"
	case NordTheme:
		return "https://cdnjs.cloudflare.com/ajax/libs/mini.css/3.0.1/mini-nord.min.css"
	}

	return ""
}

type _renderctx struct{}

var renderctxkey = _renderctx{}

func ThemeM(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var theme = DefaultTheme

		if c, err := r.Cookie("theme"); err == nil {
			switch c.Value {
			case "light":
				theme = LightTheme
			case "dark":
				theme = DarkTheme
			case "nord":
				theme = NordTheme
			}
		}

		next.ServeHTTP(
			w,
			r.WithContext(context.WithValue(r.Context(), renderctxkey, theme)),
		)
	})
}

func GetTheme(ctx context.Context) Theme {
	if v, ok := ctx.Value(renderctxkey).(Theme); ok {
		return v
	}
	return DefaultTheme
}

func SetThemeCookie(w http.ResponseWriter, theme Theme) {
	SetWeakCookie(w, "theme", theme.String())
}

func handleSetTheme(w http.ResponseWriter, r *http.Request) {
	SetThemeCookie(w, ParseTheme(r.FormValue("theme")))

	// https://developer.mozilla.org/en-US/docs/Web/HTTP/Redirections
	http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
}
