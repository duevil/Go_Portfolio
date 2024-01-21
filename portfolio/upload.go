package main

import (
	"archive/zip"
	"github.com/gin-gonic/gin"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"time"
)

// handleUpload handles file upload requests; the uploaded file is handled
// according to is type, which is based on its Filetype:
//   - ZIP files are read and all containing files are handled separately in their respective ways
//   - Markdown and Asset files are uploaded to the GridFS database
//   - Static files are copied to the staticDir
//   - Templated files are copied to the tmplDir
func handleUpload(c *gin.Context) {
	log.Println("Upload requested [", c.FullPath(), "]")

	// check file
	ff, err := c.FormFile("file")
	if errStatus(c, http.StatusBadRequest, err) {
		return
	}
	log.Println("Upload request:", ff.Filename)

	// open file
	f, err := ff.Open()
	defer func(f multipart.File) { _ = f.Close() }(f)
	if errStatus(c, http.StatusInternalServerError, err) {
		return
	}

	var location string

	// handle file according to extension
	switch filetypeFromFile(ff.Filename) {
	case ZIP:
		err = handleUploadZip(ff.Size, f)
		location = "/admin/list"
	case Markdown:
		err = handleUploadMarkdownAndAsset(ff.Filename, Markdown, time.Now(), f)
		location = path.Join(pageURL, ff.Filename)
	case Static:
		err = handleUploadStatic(ff.Filename, f)
		location = path.Join(staticURL, ff.Filename)
	case Templated:
		err = handleUploadTemplated(ff.Filename, f)
		location = path.Join(tmplURL, ff.Filename)
	case Asset:
		err = handleUploadMarkdownAndAsset(ff.Filename, Asset, time.Now(), f)
		location = path.Join(assetURL, ff.Filename)
	}

	// finish
	if !errStatus(c, http.StatusInternalServerError, err) {
		c.Status(http.StatusCreated)
		c.Header("Location", location)
	}
}

// handleUploadZip handles ZIP file uploads; the uploaded file is read and all
// containing files are handled separately in their respective ways
func handleUploadZip(size int64, rc io.ReaderAt) error {
	log.Println("Handling ZIP upload")

	// create zip reader
	zr, err := zip.NewReader(rc, size)
	if err != nil {
		return err
	}

	// process all files contained in zip
	for _, f := range zr.File {
		// open file
		rc, err := f.Open()
		// close is done inside the corresponding method
		if err != nil {
			return err
		}

		// process file according to file extension
		switch filetypeFromFile(f.Name) {
		case Markdown:
			err = handleUploadMarkdownAndAsset(f.Name, Markdown, f.Modified, rc)
		case Static:
			err = handleUploadStatic(f.Name, rc)
		case Templated:
			err = handleUploadTemplated(f.Name, rc)
		case Asset:
			err = handleUploadMarkdownAndAsset(f.Name, Asset, f.Modified, rc)
		}

		if err != nil {
			return err
		}
	}

	return nil
}

// handleUploadMarkdownAndAsset handles Markdown and Asset file uploads; the
// uploaded file is uploaded to the GridFS database
func handleUploadMarkdownAndAsset(name string, fType Filetype, modified time.Time, rc io.ReadCloser) error {
	log.Println("Handling MARKDOWN/ASSET upload:", name)
	defer rc.Close()

	// upload file to database
	file := GridFSFile{
		Name: name,
		MetaData: MetaData{
			Type:     fType,
			Modified: modified,
		},
	}
	return file.Upload(rc)
}

// handleUploadStatic handles Static file uploads; the uploaded file is copied
// to the staticDir
func handleUploadStatic(name string, rc io.ReadCloser) error {
	log.Println("Handling STATIC upload:", name)
	defer rc.Close()

	// create file in static dir
	f, err := os.Create(path.Join(staticDir, name))
	if err != nil {
		return err
	}

	// copy contents
	_, err = io.Copy(f, rc)
	return err
}

// handleUploadTemplated handles Templated file uploads; the uploaded file is
// copied to the tmplDir
func handleUploadTemplated(name string, rc io.ReadCloser) error {
	log.Println("Handling TEMPLATED upload:", name)
	defer rc.Close()

	// create file in tmpl dir
	f, err := os.Create(path.Join(tmplDir, name))
	if err != nil {
		return err
	}

	// copy contents
	_, err = io.Copy(f, rc)
	return err
}

/*
func handleUploadZip(c *gin.Context) {
	log.Println("Zip upload requested from", c.FullPath())
	// check file
	ff, err := c.FormFile("file")
	if errStatus(c, http.StatusBadRequest, err) {
		return
	}
	log.Println("Upload request:", ff.Filename)
	if path.Ext(ff.Filename) != ".zip" &&
		errStatus(c, http.StatusBadRequest, errors.New("only zip files are allowed")) {
		return
	}
	// create temp dir
	dir, err := os.MkdirTemp("", "Go_Portfolio")
	if errStatus(c, http.StatusInternalServerError, err) {
		return
	}
	defer func(path string) { _ = os.RemoveAll(path) }(dir)
	// save file
	fp := path.Join(dir, ff.Filename)
	err = c.SaveUploadedFile(ff, fp)
	if errStatus(c, http.StatusInternalServerError, err) {
		return
	}
	err = decompressDir(dir, fp)
	if errStatus(c, http.StatusInternalServerError, err) {
		return
	}
	// delete zip file to prevent processing it
	err = os.Remove(fp)
	if errStatus(c, http.StatusInternalServerError, err) {
		return
	}
	// process files
	configs, err := files.ProcessDir(ctx, bucket, dir)
	if errStatus(c, http.StatusInternalServerError, err) {
		return
	}
	for _, config := range configs {
		// if config has a predefined url, add it to the router
		if config.PredefinedURL {
			switch config.FileType {
			case files.Asset:
				router.GET(config.URL, handleAsset)
			case files.Markdown:
				router.GET(config.URL, handlePage)
			case files.Static:
				router.StaticFile(config.URL, path.Join(files.StaticDir, config.URL))
			default:
				log.Println("Unknown file type:", config.FileType)
				log.Println("Skipping route for", config.URL)
			}
		}
	}
}

func handleUploadSingle(c *gin.Context) {
	log.Println("File upload requested from", c.FullPath())
	// check file
	ff, err := c.FormFile("file")
	if errStatus(c, http.StatusBadRequest, err) {
		return
	}
	log.Println("Upload request:", ff.Filename)
	// create temp dir
	dir, err := os.MkdirTemp("", "Go_Portfolio")
	if errStatus(c, http.StatusInternalServerError, err) {
		return
	}
	defer func(path string) { _ = os.RemoveAll(path) }(dir)
	// save file
	fp := path.Join(dir, ff.Filename)
	err = c.SaveUploadedFile(ff, fp)
	if errStatus(c, http.StatusInternalServerError, err) {
		return
	}
	// open file
	f, err := os.Open(fp)
	if errStatus(c, http.StatusInternalServerError, err) {
		return
	}
	defer func(f *os.File) { _ = f.Close() }(f)
	// process file
	config, err := files.ProcessSingle(ctx, bucket, f)
	if errStatus(c, http.StatusInternalServerError, err) {
		return
	}
	c.Status(http.StatusCreated)
	c.Header("Location", config.URL)
}
*/
