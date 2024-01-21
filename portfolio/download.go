package main

import (
	"archive/zip"
	"github.com/gin-gonic/gin"
	"html/template"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const (
	zipName = "go_portfolio.zip"
)

// handleDownload handles file download requests; the requested files are
// compressed into a zip file and sent to the client
func handleDownload(c *gin.Context) {
	log.Println("Download requested [", c.FullPath(), "]")

	// create zip file
	tmpDir, err := os.MkdirTemp("", "tmp")
	if errStatus(c, http.StatusInternalServerError, err) {
		return
	}
	defer func(path string) { _ = os.RemoveAll(path) }(tmpDir)
	zp := path.Join(tmpDir, zipName)
	zf, err := os.Create(zp)
	zw := zip.NewWriter(zf)
	defer func(zw *zip.Writer) { _ = zw.Close() }(zw)

	log.Println("Writing to zip file '", zf.Name(), "'")

	err = handleDownloadStatic(zw)
	if errStatus(c, http.StatusInternalServerError, err) {
		return
	}

	err = handleDownloadMarkdown(zw)
	if errStatus(c, http.StatusInternalServerError, err) {
		return
	}

	err = handleDownloadAsset(zw)
	if errStatus(c, http.StatusInternalServerError, err) {
		return
	}

	err = handleDownloadTemplated(zw)
	if errStatus(c, http.StatusInternalServerError, err) {
		return
	}

	log.Println("Sending file")
	c.FileAttachment(zp, zipName)
}

// handleDownloadStatic writes all static files to the given zip.Writer,
// i.e. it writes all files from the static directory to the zip file
func handleDownloadStatic(zw *zip.Writer) error {
	log.Println("Writing static files to zip")

	return filepath.WalkDir(staticDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		w, err := zw.Create(path.Join(staticURL, d.Name()))
		if err != nil {
			return err
		}
		r, err := os.Open(p)
		if err != nil {
			return err
		}
		defer r.Close()
		_, err = io.Copy(w, r)
		return err
	})
}

// handleDownloadMarkdown writes all markdown files to the given zip.Writer,
// i.e. it downloads all markdown files from the database and writes them to the
// zip file
func handleDownloadMarkdown(zw *zip.Writer) error {
	log.Println("Writing markdown files to zip")

	tmpl, err := template.ParseGlob(path.Join(tmplDir, "*.*"))
	if err != nil {
		return err
	}

	files, err := ListGridFSFiles()
	if err != nil {
		return err
	}
	for _, f := range files {
		// if file is a markdown file, parse template and write to zip
		if f.MetaData.Type == Markdown {
			data, err := f.MarkdownToHTML()
			if err != nil {
				return err
			}
			w, err := zw.Create(path.Join(pageURL, f.Name))
			if err != nil {
				return err
			}
			err = tmpl.ExecuteTemplate(w, "page", data)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// handleDownloadAsset writes all asset files to the given zip.Writer,
// i.e. it downloads all assets from the database and writes them to the zip
// file
func handleDownloadAsset(zw *zip.Writer) error {
	log.Println("Writing asset files to zip")

	files, err := ListGridFSFiles()
	if err != nil {
		return err
	}
	for _, f := range files {
		// if file is an asset, download to zip file
		if f.MetaData.Type == Asset {
			w, err := zw.Create(path.Join(assetURL, f.Name))
			if err != nil {
				return err
			}
			err = f.Download(w)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// handleDownloadTemplated writes all templated files to the given zip.Writer,
// i.e. it parses all templates contained in the templates directory and writes
// the result to the zip file
func handleDownloadTemplated(zw *zip.Writer) error {
	log.Println("Writing templated files to zip")

	tmpl, err := template.ParseGlob(path.Join(tmplDir, "*.*"))
	if err != nil {
		return err
	}

	return filepath.WalkDir(tmplDir, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		// parse template and write to zip
		name, _ := strings.CutSuffix(d.Name(), path.Ext(d.Name()))
		w, err := zw.Create(path.Join(tmplURL, name+".html"))
		if err != nil {
			return err
		}
		return tmpl.ExecuteTemplate(w, name, nil)
	})
}

/*
	// get and files
	_, err := files.ListFromDB(ctx, bucket)
	if errStatus(c, http.StatusInternalServerError, err) {
		return
	}

	// create temp dir
	dir, err := os.MkdirTemp("", "Go_Portfolio")
	if errStatus(c, http.StatusInternalServerError, err) {
		return
	}
	defer func(path string) { _ = os.RemoveAll(path) }(dir)
	err = createStaticFiles(dir)
	if errNotFound(c, err) || errStatus(c, http.StatusInternalServerError, err) {
		return
	}

	// compress dir to zip file
	zipName := "Go_Portfolio.zip"
	err = compressDir(dir, path.Join(dir, zipName))
	if errStatus(c, http.StatusInternalServerError, err) {
		return
	}

	// send zip file
	c.FileAttachment(path.Join(dir, zipName), zipName)
*/
