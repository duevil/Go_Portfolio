package main

import (
	"archive/zip"
	"bytes"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"time"
)

// handleUpload handles requests for uploading files; the uploaded file is
// handled according to its extension, i.e. static files are moved to the static
// directory, templates are moved to the template directory, markdown files are
// stored in the database and zip files read and its contents iterated over and
// then handled respectively
//
// Due to unknown reasons, the auth middleware does not work with the upload of
// larger files and thus, the auth middleware is called manually after the
// uploaded file has been saved
func handleUpload(c *gin.Context, auth gin.HandlerFunc) {
	log.Println("Upload requested")
	ff, err := c.FormFile("file")
	if errStatus(c, http.StatusBadRequest, err) {
		return
	}

	// create tmp dir and save file
	log.Println("Saving file:", ff.Filename)
	dir, err := os.MkdirTemp("", "tmp")
	if errISE(c, err) {
		return
	}
	defer func(path string) { _ = os.RemoveAll(path) }(dir)
	fPath := path.Join(dir, ff.Filename)
	err = c.SaveUploadedFile(ff, fPath)
	if errISE(c, err) {
		return
	}

	// check credentials
	auth(c)
	if c.IsAborted() {
		return
	}

	f, err := os.Open(fPath)
	if errISE(c, err) {
		return
	}
	// handle file according to its extension
	var location string
	switch {
	case Static.MatchesExt(ff.Filename):
		location = path.Join(StaticURL, ff.Filename)
		err = handleUploadStatic(ff.Filename, f)
	case Template.MatchesExt(ff.Filename):
		location = path.Join(TmplURL, ff.Filename)
		err = handleUploadTmpl(ff.Filename, f)
	case Markdown.MatchesExt(ff.Filename):
		location = path.Join(PageURL, ff.Filename)
		modTime := time.Now()
		fi, _err := f.Stat()
		if _err == nil {
			modTime = fi.ModTime()
		}
		err = handleUploadPage(ff.Filename, f, modTime)
	case Zipped.MatchesExt(ff.Filename):
		err = handleUploadZip(ff.Size, f)
		location = "/admin/list"
	}
	if errISE(c, err) {
		return
	}

	// finish
	c.Status(http.StatusCreated)
	c.Header("Location", location)
}

// handleUploadZip handles the upload of a zip file; iterates over the files in
// the zip file and handles them according to their extension
func handleUploadZip(size int64, f *os.File) error {
	log.Println("Handling upload of zip file:", f.Name())
	defer func() { _ = f.Close() }()
	zr, err := zip.NewReader(f, size)
	if err != nil {
		return err
	}
	for _, zf := range zr.File {
		rc, err := zf.Open()
		if err != nil {
			return err
		}
		if zf.FileInfo().IsDir() {
			continue
		}
		name := path.Base(zf.Name)
		switch {
		case Static.MatchesExt(name):
			err = handleUploadStatic(name, rc)
		case Template.MatchesExt(name):
			err = handleUploadTmpl(name, rc)
		case Markdown.MatchesExt(name):
			err = handleUploadPage(name, rc, zf.Modified)
		}
		if err != nil {
			return err
		}
	}
	return err
}

// handleUploadStatic handles the upload of a static file; the file is copied from
// the given reader to the StatDir
func handleUploadStatic(name string, rc io.ReadCloser) error {
	log.Println("Handling upload of static file:", name)
	defer func() { _ = rc.Close() }()
	f, err := os.Create(path.Join(StatDir, name))
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	_, err = io.Copy(f, rc)
	return err
}

// handleUploadTmpl handles the upload of a template; the template is copied from
// the given reader to the TmplDir
func handleUploadTmpl(name string, rc io.ReadCloser) error {
	log.Println("Handling upload of template file:", name)
	defer func() { _ = rc.Close() }()
	f, err := os.Create(path.Join(TmplDir, name))
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	_, err = io.Copy(f, rc)
	return err
}

// handleUploadPage handles the upload of a markdown page; reads the content of
// the page from the given reader and write the page to the database
func handleUploadPage(name string, rc io.ReadCloser, mod time.Time) error {
	log.Println("Handling upload of page:", name)
	defer func() { _ = rc.Close() }()
	buf := bytes.Buffer{}
	_, err := io.Copy(&buf, rc)
	if err != nil {
		return err
	}
	p := PageDataDB{
		Name:    name,
		Content: primitive.Binary{Data: buf.Bytes()},
		LastMod: mod,
	}
	p.TrimExt()
	return p.WriteToDB()
}
