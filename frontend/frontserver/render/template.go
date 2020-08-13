package render

import (
	"bytes"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/markbates/pkger"
	"github.com/tdewolff/minify"
	"github.com/tdewolff/minify/css"
	"github.com/tdewolff/minify/html"
)

// runtime minifier
var minifier = func() (minifier *minify.M) {
	minifier = minify.New()
	minifier.AddFunc("text/css", css.Minify)
	minifier.AddFunc("text/html", html.Minify)
	return
}()

var globalFns = template.FuncMap{
	// htmlTime formats the given time to input date value's format.
	"htmlTime": func(t time.Time) string {
		return t.Format("2006-01-02T15:04")
	},
	"unixNano": func(i int64) time.Time {
		return time.Unix(0, i)
	},
	"humanizeSize": func(bytes int64) string {
		return humanize.Bytes(uint64(bytes))
	},
	"humanizeNumber": func(number int) string {
		return humanize.Comma(int64(number))
	},
	"humanizeTime": func(t time.Time) string {
		return humanize.Time(t)
	},
}

type Component struct {
	Template   string
	Components map[string]Component
	Functions  template.FuncMap
}

type Page struct {
	Template   string
	Components map[string]Component
	Functions  template.FuncMap
}

func BuildPage(n string, p Page) *Template {
	// Combine all component duplicates.
	for _, component := range p.Components {
		if component.Components != nil {
			for n, component := range component.Components {
				p.Components[n] = component
			}
		}
	}

	// Combine all function duplicates.
	for _, component := range p.Components {
		if component.Functions != nil {
			// Ensure that we have a parent functions map.
			if p.Functions == nil {
				p.Functions = template.FuncMap{}
			}

			for n, fn := range component.Functions {
				// Only set into the map if we don't already have the function.
				if _, ok := p.Functions[n]; !ok {
					p.Functions[n] = fn
				}
			}
		}
	}

	tmpl := template.New(n)
	tmpl = tmpl.Funcs(globalFns)
	tmpl = tmpl.Funcs(p.Functions)
	tmpl = template.Must(tmpl.Parse(string(read(p.Template))))

	// Parse all components' HTMLs.
	for n, component := range p.Components {
		tmpl = template.Must(tmpl.Parse(
			fmt.Sprintf(
				"{{ define \"%s\" }}%s{{ end }}",
				n, string(read(component.Template)),
			),
		))
	}

	return &Template{tmpl}
}

var index = template.Must(
	template.
		New("index").
		Parse(string(read(
			pkger.Include("/frontend/frontserver/pages/index.html"),
		))),
)

type Template struct {
	*template.Template
}

func (t *Template) Render(v interface{}) template.HTML {
	var b bytes.Buffer

	if err := t.Execute(&b, v); err != nil {
		// TODO
		log.Println("Template error:", err)
		return template.HTML("oh no")
	}

	return template.HTML(b.String())
}

func init() {
	RegisterCSSFile(pkger.Include("/frontend/frontserver/pages/style.css"))
}

var componentsCSS bytes.Buffer
var componentModTime time.Time

// RegisterCSSFile adds the CSS file to the global CSS file, which can be
// located in /components.css
func RegisterCSSFile(pkgerPath string) {
	s, err := pkger.Stat(pkgerPath)
	if err == nil {
		if modt := s.ModTime(); modt.After(componentModTime) {
			componentModTime = s.ModTime()
		}
	}

	c, err := minifier.Bytes("text/css", read(pkgerPath))
	if err != nil {
		log.Panicln("Failed to add minifying CSS:", err)
	}
	componentsCSS.Write(c)
}

func componentsCSSHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeContent(
		w, r, "components.css", componentModTime,
		bytes.NewReader(componentsCSS.Bytes()),
	)
}

func read(path string) []byte {
	f, err := pkger.Open(path)
	if err != nil {
		log.Fatalln("Failed to open template:", err)
	}

	b, err := ioutil.ReadAll(f)
	if err != nil {
		log.Fatalln("Failed to read template:", err)
	}

	f.Close()

	return b
}
