package footer

import (
	"github.com/diamondburned/smolboard/frontend/frontserver/render"
	"github.com/markbates/pkger"
)

func init() {
	render.RegisterCSSFile(
		pkger.Include("/frontend/frontserver/components/footer/footer.css"),
	)
}

var Component = render.Component{
	Template: pkger.Include("/frontend/frontserver/components/footer/footer.html"),
}
