package render

import (
	"bytes"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/phogolabs/parcello"
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

// prepareList is the list of templates to call prepare on.
var prepareList []*Template

func prepareAllTemplates() {
	for _, tmpl := range prepareList {
		tmpl.prepare()
	}
}

func BuildPage(n string, p Page) *Template {
	tmpl := &Template{
		name: n,
		page: p,
	}

	prepareList = append(prepareList, tmpl)

	return tmpl
}

type Template struct {
	*template.Template
	name string
	page Page
	once sync.Once
}

func (t *Template) prepare() {
	t.once.Do(t.do)
}

func (t *Template) do() {
	// Combine all component duplicates.
	for _, component := range t.page.Components {
		if component.Components != nil {
			for n, component := range component.Components {
				t.page.Components[n] = component
			}
		}
	}

	// Combine all function duplicates.
	for _, component := range t.page.Components {
		if component.Functions != nil {
			// Ensure that we have a parent functions map.
			if t.page.Functions == nil {
				t.page.Functions = template.FuncMap{}
			}

			for n, fn := range component.Functions {
				// Only set into the map if we don't already have the function.
				if _, ok := t.page.Functions[n]; !ok {
					t.page.Functions[n] = fn
				}
			}
		}
	}

	tmpl := template.New(t.name)
	tmpl = tmpl.Funcs(globalFns)
	tmpl = tmpl.Funcs(t.page.Functions)
	tmpl = template.Must(tmpl.Parse(string(read(t.page.Template))))

	// Parse all components' HTMLs.
	for n, component := range t.page.Components {
		tmpl = template.Must(tmpl.Parse(
			fmt.Sprintf(
				"{{ define \"%s\" }}%s{{ end }}",
				n, string(read(component.Template)),
			),
		))
	}

	t.Template = tmpl
}

// Render renders the template with the given argument into HTML.
func (t *Template) Render(v interface{}) template.HTML {
	t.prepare()

	var b bytes.Buffer

	if err := t.Execute(&b, v); err != nil {
		// TODO
		log.Println("Template error:", err)
		return template.HTML("oh no")
	}

	return template.HTML(b.String())
}

var (
	componentsPath   = []string{"pages/style.css"}
	componentsCSS    = bytes.Buffer{}
	componentModTime = time.Time{}
)

// RegisterCSSFile adds the CSS file to the global CSS file, which can be
// located in /components.css
func RegisterCSSFile(path string) {
	componentsPath = append(componentsPath, path)
}

func initializeCSS() {
	for _, path := range componentsPath {
		f, err := parcello.Open(path)
		if err != nil {
			log.Fatalln("Failed to open file:", err)
		}
		defer f.Close()

		s, err := f.Stat()
		if err != nil {
			log.Fatalln("Failed to stat file:", err)
		}

		if modt := s.ModTime(); modt.After(componentModTime) {
			componentModTime = s.ModTime()
		}

		if err := minifier.Minify("text/css", &componentsCSS, f); err != nil {
			log.Panicln("Failed to add minifying CSS:", err)
		}
	}
}

func componentsCSSHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeContent(
		w, r, "components.css", componentModTime,
		bytes.NewReader(componentsCSS.Bytes()),
	)
}

func read(path string) []byte {
	f, err := parcello.Open(path)
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

var initOnce sync.Once
var index *template.Template

func ensureInit() {
	initOnce.Do(func() {
		parcello.Manager.Walk(".", func(path string, _ os.FileInfo, _ error) error {
			log.Println("Path:", path)
			return nil
		})

		index = template.Must(
			template.
				New("index").
				Parse(string(read("pages/index.html"))),
		)

		initializeCSS()
		prepareAllTemplates()
	})
}
