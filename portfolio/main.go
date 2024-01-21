package main

import (
	"context"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/gridfs"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"html/template"
	"log"
	"os"
	"path"
	"time"
)

var (
	ctx    context.Context
	bucket *gridfs.Bucket
	router *gin.Engine
)

const (
	staticDir = "./static"
	tmplDir   = "./templates"
	staticURL = "/static/"
	tmplURL   = "/tmpl/"
	pageURL   = "/pages/"
	assetURL  = "/assets/"
)

func main() {
	// database initialization
	{
		log.Println("Connecting to database")
		// open database connection
		ctx = context.Background()
		auth := options.Credential{
			Username: os.Getenv("MDB_ROOT_USERNAME"),
			Password: os.Getenv("MDB_ROOT_PASSWORD"),
		}
		opt := options.Client().ApplyURI("mongodb://mdb:27017")
		opt.SetAuth(auth)
		client, err := mongo.Connect(ctx, opt)
		checkErr(err)
		// close database connection on exit
		defer func(c *mongo.Client) { checkErr(c.Disconnect(ctx)) }(client)
		// check whether the database is reachable
		err = client.Ping(ctx, readpref.Primary())
		checkErr(err)
		log.Println("Database connection established, initializing database")
		// create database and bucket
		db := client.Database(getEnvOrElse("DB_NAME", "portfolio"))
		opts := options.GridFSBucket().SetName(getEnvOrElse("DB_GRIDFS_NAME", "portfolio_fs"))
		bucket, err = gridfs.NewBucket(db, opts)
		checkErr(err)
		log.Println("Database initialized")
	}
	// gin initialization
	{
		log.Println("Initializing server")
		// bind gin routes
		router = gin.Default()
		router.LoadHTMLGlob(path.Join(tmplDir, "*.*"))
		router.Static(staticURL, staticDir)
		router.SetFuncMap(template.FuncMap{"timeIsZero": func(t time.Time) bool { return !t.IsZero() }})
		// add routes
		router.NoRoute(handleNotFound)
		router.GET("/", handleIndex)
		router.GET("/index.html", handleIndex)
		router.GET(path.Join(pageURL, ":name"), handlePage)
		router.GET(path.Join(assetURL, ":name"), handleAsset)
		router.GET(path.Join(tmplURL, ":name"), handleTemplated)
		// add auth routes
		adminUser := getEnvOrElse("ADMIN_USERNAME", "admin")
		adminPass := getEnvOrElse("ADMIN_PASSWORD", "admin")
		auth := router.Group("/admin", gin.BasicAuth(gin.Accounts{adminUser: adminPass}))
		auth.POST("/upload", handleUpload)
		auth.GET("/download", handleDownload)
		auth.PUT("/update", handleRename)
		auth.PUT("/rename", handleRename)
		auth.GET("/list", handleList)
		auth.DELETE(path.Join(pageURL, ":name"), handleDelete)
		auth.DELETE(path.Join(assetURL, ":name"), handleDelete)
		// run server
		addr := ":" + getEnvOrElse("GIN_PORT", "9000")
		log.Println("Starting server on", addr)
		err := router.Run(addr)
		if err != nil {
			// call panic instead of fatal to allow for deferred functions to run
			log.Panicln("Error:", err)
		}
	}
	log.Println("Server stopped")
}

// getEnvOrElse returns the value for the given key if os.LookupEnv was successful
// or else returns the alternative value
func getEnvOrElse(key string, sElse string) string {
	if s, ok := os.LookupEnv(key); ok && s != "" {
		return s
	}
	return sElse
}
