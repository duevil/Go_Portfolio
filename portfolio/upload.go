package main

import (
	"archive/zip"
	"files"
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
	if path.Ext(ff.Filename) == ".zip" {
		location = "/admin/list"
		err = handleUploadZip(ff.Size, f)
	} else {
		fi, err := f.Stat()
		if errISE(c, err) {
			return
		}
		mime, err := mimetype.DetectFile(fPath)
		if errISE(c, err) {
			return
		}
		location = path.Join(files.URIRoot, ff.Filename)
		p := files.MongoFile{
			URI:      "/" + ff.Filename, // add leading slash
			Filesize: fi.Size(),
			LastMod:  fi.ModTime(),
			Mime:     mime.String(),
			IsMD:     path.Ext(ff.Filename) == ".md",
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

	iterateFunc := func(zf *zip.File) error {
		// we need to open the file twice:
		// first to get its mime type and then to store it
		rc, err := zf.Open()
		if err != nil {
			return err
		}
		defer cls(rc)
		mime, err := mimetype.DetectReader(rc)
		if err != nil {
			return err
		}
		rc.Close()
		// get file uri
		fPath, err := handleUploadZipGetUri(f.Name(), zf.Name)
		if err != nil {
			return err
		}
		// open file again and store it
		rc, err = zf.Open()
		if err != nil {
			return err
		}
		defer cls(rc)
		p := files.MongoFile{
			URI:      "/" + fPath, // add leading slash
			Filesize: int64(zf.UncompressedSize64),
			LastMod:  zf.Modified,
			Mime:     mime.String(),
			IsMD:     path.Ext(zf.FileInfo().Name()) == ".md",
		}
		return p.Store(rc)
	}

	for _, zf := range zr.File {
		if zf.FileInfo().IsDir() {
			continue
		}
		err = iterateFunc(zf)
		if err != nil {
			return err
		}
	}
	return err
}

// handleUploadZipGetUri returns the uri for the given zip file and file name;
// the zip file name is stripped of its extension and the file name is made
// relative to the zip file name
func handleUploadZipGetUri(zipName, fileName string) (string, error) {
	zipName = path.Base(zipName)
	zipName = zipName[:len(zipName)-len(path.Ext(zipName))]
	fPath, err := filepath.Rel(zipName, fileName)
	if err != nil {
		return "", err
	}
	// remove ../ from path
	if strings.HasPrefix(fPath, "..") {
		fPath, err = filepath.Rel("..", fPath)
		if err != nil {
			return "", err
		}
	}
	return fPath, nil
}
