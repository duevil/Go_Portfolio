package main

import (
	"archive/zip"
	"content"
	"github.com/gin-gonic/gin"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
)

// handleDownload handles requests for downloading the portfolio; collects all
// files from the database and writes them to a zip file, which is then served to
// the client
func handleDownload(c *gin.Context) {
	log.Println("Download requested")

	// create tmp dir and zip file
	dir, err := os.MkdirTemp("", "tmp")
	if errISE(c, err) {
		return
	}
	defer func(path string) { _ = os.RemoveAll(path) }(dir)
	fPath := path.Join(dir, "portfolio.zip")
	f, err := os.Create(fPath)
	if errISE(c, err) {
		return
	}
	defer cls(f)
	w := zip.NewWriter(f)
	defer cls(w)

	// add files
	log.Println("Collecting files to zip:", fPath)
	fs, err := content.ListAll()
	if errISE(c, err) {
		return
	}
	for _, f := range fs {
		err = handleDownloadAddFile(w, f)
		if errISE(c, err) {
			return
		}
	}

	// finish
	err = w.Close()
	if errISE(c, err) {
		return
	}
	log.Println("Serving zip file")
	c.FileAttachment(fPath, "portfolio.zip")
}

// handleDownloadAddFile adds the given file to the given zip writer; if the file
// is a markdown file, it is converted to HTML and written to the zip writer,
// else the file is written as-is
func handleDownloadAddFile(w *zip.Writer, f content.MongoFile) error {
	log.Println("Adding file to zip:", f.URI)
	// create header
	h, err := zip.FileInfoHeader(&f)
	if err != nil {
		return err
	}
	if path.Base(f.Name()) == "index.html" {
		h.Name = "index.html"
	} else {
		h.Name = filepath.ToSlash(path.Join(content.URIRoot, f.Name()))
	}
	h.Method = zip.Deflate
	zf, err := w.CreateHeader(h)
	if err != nil {
		return err
	}
	// write file
	if f.IsMD {
		page, err := f.ToPage()
		if err != nil {
			return err
		}
		err = page.CreateHTML(templates, zf)
		if err != nil {
			return err
		}
		return nil
	}
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer cls(rc)
	_, err = io.Copy(zf, rc)
	if err != nil {
		return err
	}
	return nil
}
