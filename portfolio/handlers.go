package main

import (
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"time"
)

// handleNotFound handles requests for non-existing routes; servers a 404
// response with the parsed '404' template as content
func handleNotFound(c *gin.Context) {
	log.Println("Route not found")
	c.HTML(http.StatusNotFound, "404", gin.H{})
}

// handleIndex handles requests for the index page:
//  1. tries to serve the index page from the database
//  2. if no index page is found in the database, tries to serve the static
//     index file
//  3. if no static index file is found, serves the index template
func handleIndex(c *gin.Context) {
	log.Println("Index requested")

	// search index page
	data := PageDataDB{Name: "index"}
	page, err := data.ToPage()
	if err == nil { // page found
		log.Println("Serving index page")
		c.HTML(http.StatusOK, "page", page)
		return
	}
	// handle error only if it is not file not found
	if !isNotFound(err) && errISE(c, err) {
		return
	}

	// page not found, search for static index file
	p := path.Join(StatDir, index)
	_, err = os.Stat(p)
	if err == nil { // static index file found
		log.Println("Serving static index")
		c.File(p)
		return
	}
	// handle error only if it is not file not found
	if !os.IsNotExist(err) && errISE(c, err) {
		return
	}

	// static index file not found, serve index template
	log.Println("Serving index template")
	c.HTML(http.StatusOK, "index", gin.H{})
}

// handlePage handles requests for pages; reads the requested page from the
// database and serves it as content of the parsed 'page' template
func handlePage(c *gin.Context) {
	name := c.Param("name")
	log.Println("Page requested:", name)
	pData := PageDataDB{Name: name}
	pData.TrimExt()
	p, err := pData.ToPage()
	if errNotFound(c, err) || errISE(c, err) {
		return
	}
	c.HTML(http.StatusOK, "page", p)
}

// handleTemplated handles requests for templated pages; serves the requested
// template with empty content
func handleTemplated(c *gin.Context) {
	name := c.Param("name")
	log.Println("Templated requested:", name)
	c.HTML(http.StatusOK, name, gin.H{})
}

// handleList handles requests to list all pages, templates and static files
func handleList(c *gin.Context) {
	log.Println("List requested")

	type ListEntry struct {
		Name     string    `json:"name"`
		Link     string    `json:"link"`
		Modified time.Time `json:"modified"`
	}
	var list []ListEntry

	// list all static files
	log.Println("Listing static files")
	err := filepath.WalkDir(StatDir, func(_ string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		fi, err := d.Info()
		if err != nil {
			return err
		}
		list = append(list, ListEntry{
			Name:     d.Name(),
			Link:     path.Join(StaticURL, d.Name()),
			Modified: fi.ModTime(),
		})
		return nil
	})
	if errISE(c, err) {
		return
	}

	// list all pages
	log.Println("Listing pages")
	pages, err := listAllPages()
	if errISE(c, err) {
		return
	}
	for _, p := range pages {
		list = append(list, ListEntry{
			Name:     p.Name,
			Link:     path.Join(PageURL, p.Name),
			Modified: p.LastMod,
		})
	}

	c.JSON(http.StatusOK, list)
}

// handleDelete handles requests to delete pages, templates and static files;
// deletes the requested file from the database or file system; if the file is
// not found, responds with http.StatusNotFound, else with http.StatusNoContent
func handleDelete(c *gin.Context) {
	name := c.Param("name")
	log.Println("Delete requested:", name)
	var err error
	switch {
	case Static.MatchesExt(name):
		log.Println("Deleting static file:", name)
		err = os.Remove(path.Join(StatDir, name))
	case Template.MatchesExt(name):
		log.Println("Deleting template file:", name)
		err = os.Remove(path.Join(TmplDir, name))
	case Markdown.MatchesExt(name):
		log.Println("Deleting page:", name)
		p := PageDataDB{Name: name}
		err = p.Delete()
	}
	if errNotFound(c, err) || errISE(c, err) {
		return
	}
	c.Status(http.StatusNoContent)
}
