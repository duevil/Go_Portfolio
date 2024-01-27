package main

import (
	"errors"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/mongo"
	"log"
	"net/http"
	"os"
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

// errISE checks whether the given error is not nil; if the error is not nil,
// it is logged using log.Println and c.AbortWithError is called with the
// status code http.StatusInternalServerError
func errISE(c *gin.Context, err error) bool {
	if err != nil {
		log.Println("[Err] Internal server error:", err)
		_ = c.AbortWithError(http.StatusInternalServerError, err)
		return true
	}
	return false
}

// errNotFound checks whether the given error is ErrNotFound; if the error is
// ErrNotFound, it is logged using log.Println and handleNotFound is called
func errNotFound(c *gin.Context, err error) bool {
	if isNotFound(err) {
		log.Println("[Err] Not found:", err)
		handleNotFound(c)
		return true
	}
	return false
}

// isNotFound checks whether the given error is mongo.ErrNoDocuments
func isNotFound(err error) bool {
	return errors.Is(err, mongo.ErrNoDocuments) || os.IsNotExist(err)
}
