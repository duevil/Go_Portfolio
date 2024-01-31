package main

import (
	"archive/zip"
	"content"
	"github.com/gabriel-vasile/mimetype"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// handleUpload handles requests for uploading files; if the uploaded file is a
// zip file, it is extracted and all files in the zip file are iterated over and
// stored in the database using the zip directory structure; else the file is
// just stored in the database
//
// Due to unknown reasons using an auth middleware with the upload of smaller
// files like singular markdown files works, but not with larger files like zip
// files or images, and thus the auth middleware has to be called manually after the
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

	// open file
	f, err := os.Open(fPath)
	if errISE(c, err) {
		return
	}
	defer cls(f)

	// handle file according to its extension
	var location string
	ext := path.Ext(ff.Filename)
	if ext == ".zip" {
		location = "/admin/list"
		err = handleUploadZip(ff.Size, f)
	} else {
		fi, err := f.Stat()
		if errISE(c, err) {
			return
		}
		ok, mime := checkMimeType(ext)
		if !ok {
			mt, err := mimetype.DetectFile(fPath)
			mime = mt.String()
			if errISE(c, err) {
				return
			}
		}
		location = path.Join(content.URIRoot, ff.Filename)
		p := content.MongoFile{
			URI:      "/" + ff.Filename, // add leading slash
			Filesize: fi.Size(),
			LastMod:  fi.ModTime(),
			Mime:     mime,
			IsMD:     ext == ".md",
		}
		err = p.Store(f)
	}
	if errISE(c, err) {
		return
	}

	// finish
	c.Status(http.StatusCreated)
	c.Header("Location", location)
}

// handleUploadZip handles the upload of a zip file; iterates over the files in
// the zip file and stores them in the database
func handleUploadZip(size int64, f *os.File) error {
	log.Println("Handling upload of zip file:", f.Name())
	zr, err := zip.NewReader(f, size)
	if err != nil {
		return err
	}
	// iterate over files in zip file
	for _, zf := range zr.File {
		if zf.FileInfo().IsDir() {
			continue
		}
		err = handleUploadZipIterateFunc(f.Name(), zf)
		if err != nil {
			return err
		}
	}
	return err
}

// handleUploadZipIterateFunc is the function that is called for each file in
// the zip file
func handleUploadZipIterateFunc(fName string, zf *zip.File) error {
	// set mime type
	ext := path.Ext(zf.FileInfo().Name())
	ok, mime := checkMimeType(ext)
	if !ok {
		// open file to detect mime type
		rc, err := zf.Open()
		if err != nil {
			return err
		}
		defer cls(rc)
		mt, err := mimetype.DetectReader(rc)
		mime = mt.String()
		if err != nil {
			return err
		}
		rc.Close()
	}
	// get file uri
	fPath := path.Base(fName)
	fPath = fPath[:len(fPath)-len(path.Ext(fPath))]
	fPath, err := filepath.Rel(fPath, zf.Name)
	if err != nil {
		return err
	}
	// remove ../ from path
	if strings.HasPrefix(fPath, "..") {
		fPath, err = filepath.Rel("..", fPath)
		if err != nil {
			return err
		}
	}
	if err != nil {
		return err
	}
	// open file again and store it
	rc, err := zf.Open()
	if err != nil {
		return err
	}
	defer cls(rc)
	p := content.MongoFile{
		URI:      "/" + fPath, // add leading slash
		Filesize: int64(zf.UncompressedSize64),
		LastMod:  zf.Modified,
		Mime:     mime,
		IsMD:     ext == ".md",
	}
	return p.Store(rc)
}

// checkMimeType checks if the given extension is a valid extension and returns
// the mime type for the extension
func checkMimeType(ext string) (bool, string) {
	switch ext {
	case ".md":
		return true, "text/markdown; charset=utf-8"
	case ".html":
		return true, "text/html; charset=utf-8"
	case ".css":
		return true, "text/css; charset=utf-8"
	case ".js":
		return true, "application/javascript; charset=utf-8"
	case ".jpg", ".jpeg":
		return true, "image/jpeg"
	case ".png":
		return true, "image/png"
	case ".gif":
		return true, "image/gif"
	case ".svg":
		return true, "image/svg+xml"
	case ".ico":
		return true, "image/vnd.microsoft.icon"
	case ".webp":
		return true, "image/webp"
	case ".pdf":
		return true, "application/pdf"
	case ".zip":
		return true, "application/zip"
	case ".json":
		return true, "application/json"
	case ".xml":
		return true, "application/xml"
	case ".txt":
		return true, "text/plain; charset=utf-8"
	default:
		return false, ""
	}
}
