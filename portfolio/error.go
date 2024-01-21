package main

import (
	"errors"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/mongo/gridfs"
	"log"
)

// checkErr checks whether the given error is not nil; if the error is not nil,
// it is logged using log.Fatalln
func checkErr(err error) {
	if err != nil {
		log.Fatalln("Error:", err)
	}
}

// errStatus checks whether the given error is not nil; if the error is not nil,
// it is logged using log.Println and the error is returned to the client using
// c.AbortWithError with the given status code
func errStatus(c *gin.Context, status int, err error) bool {
	if err != nil {
		log.Println("[Err] Gin [", status, "]:", err)
		_ = c.AbortWithError(status, err)
		return true
	}
	return false
}

// errNotFound checks whether the given error is ErrNotFound; if the error is
// ErrNotFound, it is logged using log.Println and handleNotFound is called
func errNotFound(c *gin.Context, err error) bool {
	if errors.Is(err, gridfs.ErrFileNotFound) {
		log.Println("[Err] Not found:", err)
		handleNotFound(c)
		return true
	}
	return false
}
