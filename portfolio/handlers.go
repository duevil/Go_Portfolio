package main

import (
	"content"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"time"
)

// handleNotFound handles requests for non-existing routes; servers a 404
// response with the parsed '404' template as content
func handleNotFound(c *gin.Context) {
	log.Println("Route not found")
	c.HTML(http.StatusNotFound, "404", content.Page{
		Title: "404",
		Base:  c.Request.URL.Path[1:], // remove leading '/'
		Root:  content.URIRoot,
		Year:  time.Now().Year(),
	})
}

// handleFile handles requests for pages, templates and static files; if the
// requested file is a markdown file, it is converted to HTML and served, else
// the file is served as-is
func handleFile(c *gin.Context) {
	file := c.Param("uri")
	log.Println("File requested:", file)
	// get file from database
	f, err := content.GetFromDB(file)
	if errNotFound(c, err) || errISE(c, err) {
		return
	}
	// serve page if file is markdown
	if f.IsMD {
		log.Println("Serving markdown page:", file)
		page, err := f.ToPage()
		if errISE(c, err) {
			return
		}
		c.HTML(http.StatusOK, "page", page)
		return
	}
	// serve file as-is
	log.Println("Serving file:", file)
	rc, err := f.Open()
	if errISE(c, err) {
		return
	}
	defer cls(rc)
	c.DataFromReader(http.StatusOK, f.Filesize, f.Mime, rc, nil)
}

// handleAdmin handles requests for the admin page; serves the parsed 'admin'
// template as page
func handleAdmin(c *gin.Context) {
	log.Println("Admin requested")
	c.HTML(http.StatusOK, "admin", content.Page{
		Title: "Admin",
		Base:  "admin/",
		Root:  content.URIRoot,
		Year:  time.Now().Year(),
	})
}

// handleList handles requests to list all files in the database
func handleList(c *gin.Context) {
	log.Println("List requested")
	list, err := content.ListAll()
	if errISE(c, err) {
		return
	}
	c.JSON(http.StatusOK, list)
}

// handleDelete handles requests to delete files from the database
func handleDelete(c *gin.Context) {
	name := c.Param("uri")
	log.Println("Delete requested:", name)
	f, err := content.GetFromDB(name)
	if errNotFound(c, err) || errISE(c, err) {
		return
	}
	err = f.Delete()
	if errISE(c, err) {
		return
	}
	c.Status(http.StatusNoContent)
}
