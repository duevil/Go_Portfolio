package main

import (
	"html/template"
	"io"
	"log"
	"os"
	"path"
	"time"
)

// Page is the representation of a page that is served to the client
type Page struct {
	Title   string
	Content template.HTML
	LastMod time.Time
	Year    int
}

// CreateHTML creates the HTML representation of the page and writes it to the
// given writer
func (p *Page) CreateHTML(w io.Writer) error {
	log.Println("Creating HTML for page:", p.Title)
	tmpl, err := template.ParseGlob(path.Join(TmplDir, "*.*"))
	if err != nil {
		return err
	}
	return tmpl.ExecuteTemplate(w, "page", p)
}

/* Methods for implementing the os.FileInfo interface */

func (p *Page) Name() string       { return p.Title + ".html" }
func (p *Page) Size() int64        { return int64(len(p.Content)) }
func (p *Page) Mode() os.FileMode  { return os.ModePerm }
func (p *Page) ModTime() time.Time { return p.LastMod }
func (p *Page) IsDir() bool        { return false }
func (p *Page) Sys() interface{}   { return nil }
