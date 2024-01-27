package main

import (
	"context"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"log"
	"os"
	"path"
)

const (
	StatDir = "./static"
	TmplDir = "./templates"

	StaticURL = "static"
	TmplURL   = "tmpl"
	PageURL   = "pages"

	index = "index.html"
)

var (
	ctx     context.Context
	pageCol *mongo.Collection
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
		// create database and collection
		db := client.Database(getEnvOrElse("DB_NAME", "portfolio"))
		pageCol = db.Collection(getEnvOrElse("DB_PAGE_COL", "pages"))
		log.Println("Database initialized")
	}
	// gin initialization
	{
		log.Println("Initializing server")
		// bind gin routes
		router := gin.Default()
		router.LoadHTMLGlob(path.Join(TmplDir, "*.*"))
		router.Static(StaticURL, StatDir)
		// add routes
		router.NoRoute(handleNotFound)
		router.GET("/", handleIndex)
		router.GET(index, handleIndex)
		router.GET(path.Join(PageURL, ":name"), handlePage)
		router.GET(path.Join(TmplURL, ":name"), handleTemplated)
		// add auth routes
		adminUser := getEnvOrElse("ADMIN_USERNAME", "admin")
		adminPass := getEnvOrElse("ADMIN_PASSWORD", "admin")
		// due to unknown reasons it is not possible to perform an upload of larger files when using
		// any middleware, so we must use the raw router instead and call the basic auth function
		// manually inside the handler function
		router.POST("/admin/upload", func(c *gin.Context) {
			// we pass the basic auth middleware as a handler function to the raw router
			handleUpload(c, gin.BasicAuth(gin.Accounts{adminUser: adminPass}))
		})
		auth := router.Group("/admin", gin.BasicAuth(gin.Accounts{adminUser: adminPass}))
		auth.GET("/download", handleDownload)
		auth.GET("/list", handleList)
		auth.DELETE(path.Join("/delete", ":name"), handleDelete)
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
