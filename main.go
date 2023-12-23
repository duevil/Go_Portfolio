package main

import (
	"archive/zip"
	"context"
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/russross/blackfriday/v2"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"html/template"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"time"
)

//#region variables and types

const (
	statDir = "./static"
	tmplDir = "./templates"
	tempDir = "./tmp"
	pageDir = "./pages"
)

var (
	ctx     context.Context
	pageCol *mongo.Collection
)

type PageData struct {
	ID      primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	Title   string             `bson:"title,omitempty" json:"title,omitempty"`
	Content []byte             `bson:"content,omitempty" json:"content,omitempty"`
	LastMod time.Time          `bson:"last_mod,omitempty" json:"last_mod,omitempty"`
}

type Page struct {
	Menu        []string
	Title       string
	Content     template.HTML
	OmitModTime bool
	LastMod     time.Time
	Year        int
}

//#endregion

func main() {
	// open database connection
	ctx = context.Background()
	auth := options.Credential{
		Username: os.Getenv("MDB_ROOT_USERNAME"),
		Password: os.Getenv("MDB_ROOT_PASSWORD"),
	}
	log.Println(auth)
	opt := options.Client().ApplyURI("mongodb://mdb:27017")
	opt.SetAuth(auth)
	client, err := mongo.Connect(ctx, opt)
	checkErr(err)
	// check whether the database is reachable
	err = client.Ping(ctx, readpref.Primary())
	checkErr(err)
	pageCol = client.
		Database(getEnvOrElse("DB_NAME", "portfolio")).
		Collection(getEnvOrElse("DB_PAGE_COL", "pages"))

	// bind gin routes
	router := gin.Default()
	router.LoadHTMLGlob(path.Join(tmplDir, "*.gohtml"))
	router.Static("/static", statDir)
	router.NoRoute(pageNotFoundHandler)
	router.GET("/", indexHandler)
	router.GET("/pages/:page", pageHandler)
	router.GET("/download-static", handleDownloadStatic)
	router.POST("/generate-static", handleGenerateStatic)
	router.POST("/upload", handleUpload)
	err = router.Run(":" + getEnvOrElse("PORT", "9000"))
	checkErr(err)
}

//#region page handlers

// pageNotFoundHandler handles any request that does not match any other route;
// the page's title is 'Nicht gefunden'
func pageNotFoundHandler(c *gin.Context) {
	log.Println("pageNotFoundHandler")
	menu, err := loadMenu()
	if checkInternalServerErr(c, err) {
		return
	}
	c.HTML(http.StatusNotFound, "404", Page{
		Menu:        menu,
		Title:       "Nicht gefunden",
		OmitModTime: true,
		Year:        time.Now().Year(),
	})
}

// indexHandler handles the request for the index page;
// the index page's title is 'Start'
func indexHandler(c *gin.Context) {
	log.Println("indexHandler")
	menu, err := loadMenu()
	if checkInternalServerErr(c, err) {
		return
	}
	c.HTML(http.StatusOK, "index", Page{
		Menu:        menu,
		Title:       "Start",
		OmitModTime: true,
		Year:        time.Now().Year(),
	})
}

// pageHandler handles the request for any page;
// the page's title is received from the request's path;
// if the page does not exist, the request is handled by pageNotFoundHandler
func pageHandler(c *gin.Context) {
	log.Println("pageHandler")
	title := c.Param("page")
	p, err := searchPageData(title)
	if errors.Is(err, mongo.ErrNoDocuments) {
		pageNotFoundHandler(c)
		return
	}
	if checkInternalServerErr(c, err) {
		return
	}
	// load page and set response
	menu, err := loadMenu()
	if checkInternalServerErr(c, err) {
		return
	}
	c.HTML(http.StatusOK, "page", Page{
		Menu:    menu,
		Title:   p.Title,
		Content: template.HTML(blackfriday.Run(p.Content)),
		LastMod: p.LastMod,
		Year:    time.Now().Year(),
	})
}

// loadMenu loads the menu from the database;
// the menu is a slice of strings containing the titles of all pages
func loadMenu() ([]string, error) {
	// query title only
	opts := options.Find().SetProjection(bson.M{"title": 1})
	cur, err := pageCol.Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, err
	}
	var menu []string
	// parse data and append to slice
	for cur.Next(ctx) {
		res := struct{ Title string }{}
		err = cur.Decode(&res)
		if err != nil {
			return nil, err
		}
		menu = append(menu, res.Title)
	}
	return menu, nil
}

//#endregion

//#region handleDownloadStatic

// handleDownloadStatic handles the serving of a zip file containing all static files;
func handleDownloadStatic(c *gin.Context) {
	log.Println("handleDownloadStatic")
	// TODO: move zip file creation to handleGenerateStatic
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
		<-c.Writer.CloseNotify()
		log.Println("handleDownloadStatic - deleting zip file:", zipName)
		_ = os.Remove(pGen)
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
	//		- get page data from DB
	//		- create page from template and blackfriday
	//		- parse page templates using template.ParseGlob
	//		- execute page template with page data and write to file
	//		- create zip file from static dir and page dir
	//		- send zip file to client
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
		}
		err = handleZipSingleFile(f, fi)
		if err != nil {
			return err
		}
	}
	// remove saved zip file
	return os.Remove(zipPath)
}

// handleZipSingleFile handles a single file contained in a zip archive received over an upload request;
// copies the files content into the program storage, depending on file's directory and extension:
// - files contained in any directory path with 'static' will be copied into the program static directory
// - files having an '.md' extension will be written into the program database, possibly overwriting existing data
// - all other files will be ignored
func handleZipSingleFile(f *zip.File, fi fs.FileInfo) error {
	log.Println("handleZipSingleFile:", fi.Name())
	// if file is static data, save it to static dir
	if isStat, _ := path.Match("*/static/*", f.Name); isStat {
		log.Println("handleZipSingleFile - file is static, saving to stat dir:", fi.Name())
		data, err := readZipFile(f)
		err = os.WriteFile(path.Join(statDir, fi.Name()), data, os.ModePerm)
		if err != nil {
			return err
		}
		return nil
	}
	// check if file is data
	if ext := path.Ext(fi.Name()); ext == ".md" {
		log.Println("handleZipSingleFile - file is data, writing to db:", fi.Name())
		title := fi.Name()[:len(fi.Name())-len(ext)]
		// check if page already exists to avoid duplicate entries
		p, err := searchPageData(title)
		if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
			return err
		}
		// read file data
		data, err := readZipFile(f)
		if err != nil {
			return err
		}
		p.Title = title
		p.Content = data
		p.LastMod = fi.ModTime()
		// insert or update page
		if p.ID.IsZero() {
			_, err = pageCol.InsertOne(ctx, p)
		} else {
			_, err = pageCol.ReplaceOne(ctx, bson.M{"_id": p.ID}, p)
		}
		if err != nil {
			return err
		}
		return nil
	}
	log.Println("handleZipSingleFile - file is neither static nor .md data, skipping:", fi.Name())
	return nil
}

// readZipFile reads the content of a zip file's file;
// returns the file's content as a byte slice
func readZipFile(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	data := make([]byte, f.UncompressedSize64)
	_, err = io.ReadFull(rc, data)
	if err != nil {
		return nil, err
	}
	return data, nil
}

//#endregion

//#region util methods

// checkErr checks whether the given error is not nil;
// if the error is not null, calls log.Fatal
func checkErr(err error) {
	if err != nil {
		log.Fatalln(err)
	}
}

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

// getEnvOrElse returns the value for the given key if os.LookupEnv was successful
// or else returns the alternative value
func getEnvOrElse(key string, sElse string) string {
	if s, ok := os.LookupEnv(key); ok && s != "" {
		return s
	}
	return sElse
}

// searchPageData searches the database for a page with the given title;
func searchPageData(title string) (PageData, error) {
	var p PageData
	// regex search for case-insensitive title
	r := pageCol.FindOne(ctx, bson.M{"title": primitive.Regex{Pattern: "^" + title + "$", Options: "i"}})
	err := r.Decode(&p)
	if err != nil {
		return PageData{}, err
	}
	return p, nil
}

//#endregion
