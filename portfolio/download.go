package main

import (
	"archive/zip"
	"github.com/gin-gonic/gin"
	"html/template"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
)

// handleDownload handles requests for downloading the portfolio; collects all
// static files, template files and pages and serves them as a zip file
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
	defer func(f *os.File) { _ = f.Close() }(f)
	w := zip.NewWriter(f)
	defer func(w *zip.Writer) { _ = w.Close() }(w)

	// we need to check whether an index file has been found
	// as an index file is treated differently than other files
	indexFound := false

	// add static files
	err = handleDownloadAddStaticFiles(w, &indexFound)
	if errISE(c, err) {
		return
	}

	// add page files
	err = handleDownloadAddPageFiles(w, &indexFound)
	if errISE(c, err) {
		return
	}

	// add templated index if not previously found
	if !indexFound {
		err := handleDownloadAddIndexTemplate(w)
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

// handleDownloadAddStaticFiles adds all static files to the zip file
func handleDownloadAddStaticFiles(w *zip.Writer, indexFound *bool) error {
	log.Println("Adding static files")
	return filepath.Walk(StatDir, func(fp string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		h, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		// handle index file if not previously found
		if !*indexFound && info.Name() == index {
			h.Name = index
			*indexFound = true
		} else {
			h.Name = filepath.ToSlash(path.Join(StaticURL, info.Name()))
		}
		h.Method = zip.Deflate
		f, err := w.CreateHeader(h)
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		r, err := os.Open(fp)
		if err != nil {
			return err
		}
		defer func(r *os.File) { _ = r.Close() }(r)
		_, err = io.Copy(f, r)
		return err
	})
}

// handleDownloadAddPageFiles adds all pages from the database to the zip file
func handleDownloadAddPageFiles(w *zip.Writer, indexFound *bool) error {
	log.Println("Adding page files")
	pages, err := listAllPages()
	if err != nil {
		return err
	}
	for _, p := range pages {
		page, err := p.ToPage()
		if err != nil {
			return err
		}
		h, err := zip.FileInfoHeader(&page)
		if err != nil {
			return err
		}
		// handle index file if not previously found
		if !*indexFound && page.Name() == index {
			h.Name = index
			*indexFound = true
		} else {
			h.Name = filepath.ToSlash(path.Join(PageURL, page.Name()))
		}
		h.Method = zip.Deflate
		f, err := w.CreateHeader(h)
		if err != nil {
			return err
		}
		err = page.CreateHTML(f)
		if err != nil {
			return err
		}
	}
	return nil
}

// handleDownloadAddIndexTemplate adds the templated index to the zip file
func handleDownloadAddIndexTemplate(w *zip.Writer) error {
	log.Println("Adding templated index")
	f, err := w.Create(index)
	if err != nil {
		return err
	}
	tmpl, err := template.ParseGlob(path.Join(TmplDir, "*.*"))
	if err != nil {
		return err
	}
	// check if index template exists
	tmpl = tmpl.Lookup("index")
	if tmpl == nil {
		return nil
	}
	return tmpl.Execute(f, gin.H{})
}
