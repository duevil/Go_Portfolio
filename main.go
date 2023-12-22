package main

import (
	"archive/zip"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

const (
	statDir = "./static"
	tmplDir = "./templates"
	tempDir = "./tmp"
	pageDir = "./pages"
)

func main() {
	router := gin.Default()
	//router.LoadHTMLGlob(path.Join(tmplDir, "*.html"))
	router.Static("/static", statDir)
	router.GET("/", indexHandler)
	router.GET("/pages/:page", pageHandler)
	router.GET("/download-static", handleDownloadStatic)
	router.POST("/generate-static", handleGenerateStatic)
	router.POST("/upload", handleUpload)
	// try to get port from environment variable
	port, isSet := os.LookupEnv("PORT")
	if !isSet || port == "" {
		port = "9000"
	}
	err := router.Run(":" + port)
	if err != nil {
		log.Fatal(err)
	}
}

func indexHandler(c *gin.Context) {
	log.Println("indexHandler")
	// TODO: create index page
	c.HTML(http.StatusOK, "index.html", nil)
}

func pageHandler(c *gin.Context) {
	log.Println("pageHandler")
	page := c.Param("page")
	// TODO: create page from DB
	//		- get page markdown data from DB
	//		- create page from template and blackfriday
	c.HTML(http.StatusOK, page+".html", nil)
}

//#region handleDownloadStatic

func handleDownloadStatic(c *gin.Context) {
	log.Println("handleDownloadStatic")
	// TODO: test
	zipName := "static.zip"
	log.Println("handleDownloadStatic - creating zip file:", zipName)
	// create temp dir to store zip into to
	err := os.Mkdir(tempDir, os.ModePerm)
	if checkInternalServerErr(c, err) {
		return
	}
	// create the zip archive
	pGen := path.Join(tempDir, zipName)
	f, err := os.Create(pGen)
	if checkInternalServerErr(c, err) {
		return
	}
	defer f.Close()
	w := zip.NewWriter(f)
	defer w.Close()
	// iterate over all files in stat dir and page dir and copy to zip archive
	log.Println("handleDownloadStatic - adding files to zip file:", zipName)
	err = filepath.Walk(statDir, func(p string, fi os.FileInfo, err error) error {
		return addFileToZip(p, fi, err, w, false)
	})
	err = filepath.Walk(pageDir, func(p string, fi os.FileInfo, err error) error {
		return addFileToZip(p, fi, err, w, true)
	})
	if checkInternalServerErr(c, err) {
		return
	}
	log.Println("handleDownloadStatic - sending zip file:", zipName)
	c.FileAttachment(path.Join(tempDir, zipName), zipName)
	// delete zip file after request closes (hopefully)
	go func(c *gin.Context) {
		if cChan := c.Done(); cChan != nil {
			if _, done := <-cChan; done {
				err := os.Remove(pGen)
				if err != nil {
					log.Println(err)
				}
			}
		}
	}(c)
}

// addFileToZip adds a file specified by it name and os.FileInfo to a zip file using a zip.Writer;
// if addDir is set to true, the file is added with its directory tree,
// otherwise only the file is added ti the zip file's root directory
func addFileToZip(p string, fi os.FileInfo, err error, w *zip.Writer, addDir bool) error {
	if err != nil {
		return err
	}
	if fi.IsDir() {
		return nil
	}
	log.Println("addFileToZip - adding file to zip:", p)
	var fw io.Writer
	if addDir {
		fw, err = w.Create(fi.Name())
	} else {
		fw, err = w.Create(p)
	}
	if err != nil {
		return err
	}
	f, err := os.Open(p)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(fw, f)
	return err
}

//#endregion
//#region handleGenerateStatic

func handleGenerateStatic(c *gin.Context) {
	log.Println("handleGenerateStatic")
	// TODO: generate and save static files
	//		- get page markdown data from DB
	//		- create page from template and blackfriday
	//		- save page to static dir
	//		- create download link
	c.Status(http.StatusCreated)
	c.Header("Location", "/downloadStatic")
}

//#endregion
//#region handleUpload

// handleUpload handles any upload request and the received data;
// if the received data is of type '.zip', the data is handled with handleZipData
func handleUpload(c *gin.Context) {
	log.Println("handleUpload")
	// get file from request
	f, err := c.FormFile("file")
	if err != nil {
		log.Println(err)
		c.Status(http.StatusBadRequest)
		_ = c.Error(err)
		return
	}
	log.Println("Received file:", f.Filename)
	// check file extension
	ext := path.Ext(f.Filename)
	if ext != ".zip" {
		log.Println("File extension not allowed:", ext)
		c.Status(http.StatusBadRequest)
		_ = c.Error(err)
		return
	}
	// save file to temp dir
	fp := path.Join(tempDir, f.Filename)
	err = c.SaveUploadedFile(f, fp)
	if checkInternalServerErr(c, err) {
		return
	}
	// handle zip data
	err = handleZipData(fp)
	if checkInternalServerErr(c, err) {
		return
	}
	c.Status(http.StatusNoContent)
	log.Println("file upload handling finished:", c.Writer.Status())
}

// handleZipData handles the readout and copying of a zip file's data received over an upload request;
// each file contained in the zip archive is handled with handleZipSingleFile
func handleZipData(zipPath string) error {
	log.Println("handleZipData:", zipPath)
	// open zip file for reading
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer zr.Close()
	// iterate over files in zip
	for _, f := range zr.File {
		fi := f.FileInfo()
		log.Println("handleZipData - file:", fi.Name())
		// check if file is dir
		if fi.IsDir() {
			log.Println("handleZipData - file is dir:", fi.Name())
			continue // skip dir
		} else {
			err = handleZipSingleFile(f, fi)
			if err != nil {
				return err
			}
		}
	}
	// remove saved zip file
	return os.RemoveAll(tempDir)
}

// handleZipSingleFile handles a single file contained in a zip archive received over an upload request;
// copies the files content into the program storage, depending on file's directory and extension:
// - files contained in any directory path with 'static' will be copied into the program static directory
// - files having an '.md' extension will be written into the program database
func handleZipSingleFile(f *zip.File, fi fs.FileInfo) error {
	log.Println("handleZipSingleFile:", fi.Name())
	// open file for reading
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	// read file content
	data := make([]byte, f.UncompressedSize64)
	_, err = io.ReadFull(rc, data)
	if err != nil {
		return err
	}
	// check if file is static data
	isStat, err := path.Match("*/static/*", f.Name)
	if err != nil {
		return err
	}
	// if file is static data, save it to static dir
	if isStat {
		log.Println("handleZipSingleFile - file is static, saving to stat dir:", fi.Name())
		err = os.WriteFile(path.Join(statDir, fi.Name()), data, os.ModePerm)
		if err != nil {
			return err
		}
		// else send file content to DB
	} else if ext := path.Ext(fi.Name()); ext == ".md" {
		log.Println("handleZipSingleFile - file is data, writing to db:", fi.Name())
		_ = fi.Name()[:len(fi.Name())-len(ext)]
		// TODO: write data to DB
	}
	return nil
}

//#endregion

// checkInternalServerErr checks whether the given error is not nil;
// if the error is not nil, sets the gin.Context status to http.StatusInternalServerError
// and adds the error to context
func checkInternalServerErr(c *gin.Context, err error) bool {
	if err != nil {
		log.Println(err)
		c.Status(http.StatusInternalServerError)
		_ = c.Error(err)
		return true
	}
	return false
}
