package main

import (
	"github.com/gin-gonic/gin"
	"io"
	"log"
	"net/http"
)

// handleNotFound handles requests for non-existing routes; servers a 404
// response with the parsed '404' template as content
func handleNotFound(c *gin.Context) {
	log.Println("Route not found")
	c.HTML(http.StatusNotFound, "404", gin.H{})
}

func handleFile(c *gin.Context) {
	file := c.Param("path")
	log.Println("File requested:", file)
	// get file from database
	f, err := GetFromDB(file)
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
	defer func(rc io.ReadCloser) { _ = rc.Close() }(rc)
	c.DataFromReader(http.StatusOK, f.Filesize, f.Mime, rc, nil)
}

// handleList handles requests to list all pages, templates and static files
func handleList(c *gin.Context) {
	log.Println("List requested")
	list, err := ListAllFiles()
	if errISE(c, err) {
		return
	}
	c.JSON(http.StatusOK, list)
}

// handleDelete handles requests to delete pages, templates and static files;
// deletes the requested file from the database or file system; if the file is
// not found, responds with http.StatusNotFound, else with http.StatusNoContent
func handleDelete(c *gin.Context) {
	name := c.Param("path")
	log.Println("Delete requested:", name)
	f, err := GetFromDB(name)
	if errNotFound(c, err) || errISE(c, err) {
		return
	}
	err = f.Delete()
	if errISE(c, err) {
		return
	}
	c.Status(http.StatusNoContent)
}
