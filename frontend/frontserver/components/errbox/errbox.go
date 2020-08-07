package errbox

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/diamondburned/smolboard/frontend/frontserver/render"
	"github.com/markbates/pkger"
)

func init() {
	render.RegisterCSSFile(
		pkger.Include("/frontend/frontserver/components/errbox/errbox.css"),
	)
}

var Component = render.Component{
	Template: pkger.Include("/frontend/frontserver/components/errbox/errbox.html"),
	Functions: map[string]interface{}{
		"minifyError": MinifyError,
	},
}

func MinifyError(err error) string {
	var errmsg = err.Error()
	var parts = strings.Split(errmsg, ": ")
	if len(parts) == 0 {
		return ""
	}

	var part = parts[len(parts)-1]
	// Capitalize the first letter.
	f, sz := utf8.DecodeRune([]byte(part))
	if sz > 0 {
		f = unicode.ToUpper(f)
		part = string(f) + part[sz:]
	}

	return part + "."
}
