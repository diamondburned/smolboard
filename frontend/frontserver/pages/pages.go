package pages

import "embed"

//go:embed *
var fs embed.FS

var ComponentPaths []string

// RegisterCSSFile adds the CSS file to the global CSS file, which can be
// located in /components.css
func RegisterCSSFile(path string) {
	componentsPath = append(componentsPath, path)
}
