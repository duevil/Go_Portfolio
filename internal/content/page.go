package content

import (
	"html/template"
	"io"
	"log"
	"time"
)

// Page is the representation of a page that is served to the client
type Page struct {
	Title   string
	Content template.HTML
	LastMod time.Time
	Year    int
	Base    string
	Root    string
}

// CreateHTML creates the HTML representation of the page using the given
// template and writes it to the given writer
func (p *Page) CreateHTML(tmpl *template.Template, w io.Writer) error {
	log.Println("Creating HTML for page:", p.Title)
	return tmpl.ExecuteTemplate(w, "page", p)
}
