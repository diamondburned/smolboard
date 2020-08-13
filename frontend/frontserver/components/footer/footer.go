package footer

import (
	"github.com/diamondburned/smolboard/frontend/frontserver/render"
)

func init() {
	render.RegisterCSSFile("components/footer/footer.css")
}

var Component = render.Component{
	Template: "components/footer/footer.html",
}
