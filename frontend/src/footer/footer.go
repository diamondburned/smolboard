package footer

import (
	"github.com/diamondburned/smolboard/frontend/src/footer/theme"
)

type Footer struct {
	Theme *theme.Theme
}

func NewFooter() *Footer {
	return &Footer{
		Theme: theme.NewTheme(),
	}
}
