package main

import (
	"content"
	"context"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"html/template"
	"log"
	"os"
	"path"
)

var templates = template.Must(template.ParseGlob("templates/*.*"))

func main() {
	// database initialization
	{
		log.Println("Connecting to database")
		// open database connection
		content.Context = context.Background()
		auth := options.Credential{
			Username: os.Getenv("MDB_ROOT_USERNAME"),
			Password: os.Getenv("MDB_ROOT_PASSWORD"),
		}
		opt := options.Client().ApplyURI("mongodb://mdb:27017")
		opt.SetAuth(auth)
		client, err := mongo.Connect(content.Context, opt)
		checkErr(err)
		// close database connection on exit
		defer func(c *mongo.Client) { checkErr(c.Disconnect(content.Context)) }(client)
		// check whether the database is reachable
		err = client.Ping(content.Context, readpref.Primary())
		checkErr(err)
		log.Println("Database connection established, initializing database")
		// create database and collection
		db := client.Database(getEnvOrElse("DB_NAME", "portfolio"))
		content.SetCollection(db.Collection(getEnvOrElse("DB_FILE_COL", content.URIRoot)))
		log.Println("Database initialized")
	}
	// gin initialization
	{
		log.Println("Initializing server")
		// bind gin routes
		router := gin.Default()
		router.SetHTMLTemplate(templates)
		router.NoRoute(handleNotFound)
		indexRedirect := func(c *gin.Context) {
			// handle index redirect
			c.Request.URL.Path = path.Join("/", content.URIRoot, "index.html")
			router.HandleContext(c)
		}
		router.GET("/", indexRedirect)
		router.GET("index", indexRedirect)
		router.GET("index.html", indexRedirect)
		router.GET(path.Join(content.URIRoot, "*uri"), handleFile)
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
		auth.GET("/", handleAdmin)
		auth.GET("/download", handleDownload)
		auth.GET("/list", handleList)
		auth.DELETE("*uri", handleDelete)
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
