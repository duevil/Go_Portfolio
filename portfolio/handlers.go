package main

import (
	"errors"
	"github.com/gabriel-vasile/mimetype"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/mongo/gridfs"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

// handleNotFound handles requests for non-existing routes; servers a 404
// response with the parsed '404' template as content
func handleNotFound(c *gin.Context) {
	log.Println("Route not found")
	c.HTML(http.StatusNotFound, "404", gin.H{})
}

// handleIndex handles requests for the index route; it first tries to serve the
// static index file, when failed then the index template, and finally redirects
// to the markdown index
func handleIndex(c *gin.Context) {
	log.Println("Index requested")

	// serve static index file
	p := path.Join(staticDir, "index.html")
	if _, err := os.Stat(p); err == nil {
		c.File(p)
		return
	} else if !errors.Is(err, os.ErrNotExist) && errStatus(c, http.StatusInternalServerError, err) {
		return
	} else {
		log.Println("No static index file found, serving template")
	}

	// serve index template
	c.HTML(http.StatusOK, "index", gin.H{})
	if !c.IsAborted() {
		return
	} else {
		log.Println("Index template not found, redirecting to markdown index")
	}

	// redirect to markdown index
	c.Redirect(http.StatusFound, path.Join(pageURL, "index.md"))
}

// handlePage handles requests for markdown files; the requested file is
// downloaded from the GridFS database and converted to HTML and served
func handlePage(c *gin.Context) {
	name := c.Param("name")
	log.Println("Page requested:", name)

	file := GridFSFile{Name: name}
	err := file.Read()
	if errNotFound(c, err) || errStatus(c, http.StatusInternalServerError, err) {
		return
	}
	data, err := file.MarkdownToHTML()
	if errors.Is(err, ErrNotMD) {
		// if the file is not markdown, we respond with a 404
		log.Println("Requested file is not markdown")
		err = gridfs.ErrFileNotFound
	}
	if errNotFound(c, err) || errStatus(c, http.StatusInternalServerError, err) {
		return
	}
	c.HTML(http.StatusOK, "page", data)
}

// handleAsset handles requests for asset files; the requested file is
// downloaded from the GridFS database and served
func handleAsset(c *gin.Context) {
	name := c.Param("name")
	log.Println("Asset requested:", name)

	pr, pw := io.Pipe()
	file := GridFSFile{Name: name}
	// detect the file's mimetype
	go func() {
		err := file.Download(pw)
		_ = pw.CloseWithError(err)
	}()
	mime, err := mimetype.DetectReader(pr)
	if errStatus(c, http.StatusInternalServerError, err) {
		return
	}
	// download file from database in a separate goroutine
	go func() {
		err := file.Download(pw)
		_ = pw.CloseWithError(err)
	}()
	c.DataFromReader(http.StatusOK, file.Size, mime.String(), pr, nil)
}

// handleTemplated handles requests for templated files; the requested file is
// served as a template
func handleTemplated(c *gin.Context) {
	name := c.Param("name")
	log.Println("Template requested:", name)

	tmplName, _ := strings.CutSuffix(name, path.Ext(name))
	c.HTML(http.StatusOK, tmplName, gin.H{})
}

// handleRename handles requests to rename files; the old file is renamed to the
// new file name
func handleRename(c *gin.Context) {
	log.Println("Rename requested from", c.FullPath())
	var u struct {
		OldName string `json:"old_name,omitempty"`
		NewName string `json:"new_name,omitempty"`
	}
	err := c.BindJSON(&u)
	if errStatus(c, http.StatusBadRequest, err) {
		return
	}
	log.Println("Rename request:", u)
	file := GridFSFile{Name: u.OldName}
	err = file.Rename(u.NewName)
	if errNotFound(c, err) || errStatus(c, http.StatusInternalServerError, err) {
		return
	}
	c.Status(http.StatusNoContent)
}

// handleDelete handles requests to delete files; the requested file is deleted.
// Responds with http.StatusNoContent if either the file was deleted
// successfully or was not found, e.g. due to already having been deleted
func handleDelete(c *gin.Context) {
	asset := c.Param("name")
	log.Println("File delete requested:", asset, "from", c.FullPath())
	file := GridFSFile{Name: asset}
	err := file.Delete()
	if !errors.Is(err, gridfs.ErrFileNotFound) &&
		errNotFound(c, err) ||
		errStatus(c, http.StatusInternalServerError, err) {
		return
	}
	c.Status(http.StatusNoContent)
}

// handleList handles requests to list files; lists all files contained in the
// staticDir, tmplDir and the GridFS database and returns a json document
// containing the name, modified date, and url of each file
func handleList(c *gin.Context) {
	log.Println("List requested")

	type listEntry struct {
		Name     string    `json:"name,omitempty"`
		Modified time.Time `json:"modified,omitempty"`
		URL      string    `json:"url,omitempty"`
	}
	var list []listEntry

	// list files from database
	dbFiles, err := ListGridFSFiles()
	if errStatus(c, http.StatusInternalServerError, err) {
		return
	}
	for _, f := range dbFiles {
		var url string
		switch f.MetaData.Type {
		case Markdown:
			url = path.Join(pageURL, f.Name)
		case Asset:
			url = path.Join(assetURL, f.Name)
		}
		list = append(list, listEntry{
			Name:     f.Name,
			Modified: f.MetaData.Modified,
			URL:      url,
		})
	}

	walkFunc := func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return err
		}
		list = append(list, listEntry{
			Name:     d.Name(),
			Modified: info.ModTime(),
			URL:      path,
		})
		return nil
	}

	// list files from static dir
	err = filepath.WalkDir(staticDir, walkFunc)

	// list files from templates dir
	err = filepath.WalkDir(tmplDir, walkFunc)

	c.JSON(http.StatusOK, list)
}

/*
// INDEX

	// index can be any type, so we need to check the type
	file, err := files.QueryFromDB(ctx, bucket, c.FullPath())
	if errors.Is(err, files.ErrNotFound) {
		// if the index file was not found in the database,
		// we try to serve the index template
		c.HTML(http.StatusOK, "index", gin.H{})
		return
	}
	if errStatus(c, http.StatusInternalServerError, err) {
		return
	}
	log.Println("Index type:", file.MetaData.Config.FileType)
	switch file.MetaData.Config.FileType {
	case files.Markdown, files.Asset:
		c.Redirect(http.StatusFound, file.MetaData.Config.URL)
	case files.Static:
		c.File(path.Join(files.StaticDir, file.MetaData.Config.URL))
	case files.Templated:
		c.Redirect(http.StatusFound, file.MetaData.Config.URL)
	}

// PAGE

	// get file
	file, err := files.QueryFromDB(ctx, bucket, path.Join(files.PageURL, page))
	if errNotFound(c, err) || errStatus(c, http.StatusInternalServerError, err) {
		return
	}
	// convert file to pageData
	body, err := file.ToHTML(ctx, bucket)
	if errNotFound(c, err) || errStatus(c, http.StatusInternalServerError, err) {
		return
	}
	Data := pageData{
		Title:        file.MetaData.Config.Title,
		Body:         body,
		Modified: file.MetaData.Modified,
	}
	// render page
	c.HTML(http.StatusOK, "page", Data)

// ASSET

	// open file
	file, err := files.QueryFromDB(ctx, bucket, path.Join(files.AssetURL, asset))
	if errNotFound(c, err) || errStatus(c, http.StatusInternalServerError, err) {
		return
	}
	// write file to response
	r, w := io.Pipe()
	_, err = file.Download(bucket, w)
	if errNotFound(c, err) || errStatus(c, http.StatusInternalServerError, err) {
		return
	}
	c.DataFromReader(http.StatusOK, file.Length, file.MetaData.MimeType, r, nil)

*/
