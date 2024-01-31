package main

import (
	"content"
	"errors"
	"github.com/gin-gonic/gin"
	"io"
	"log"
	"net/http"
	"os"
)

// getEnvOrElse returns the value for the given key if os.LookupEnv was successful
// or else returns the alternative value
func getEnvOrElse(key string, sElse string) string {
	if s, ok := os.LookupEnv(key); ok && s != "" {
		return s
	}
	return sElse
}

// checkErr checks whether the given error is not nil; if the error is not nil,
// it is logged using log.Fatalln
func checkErr(err error) {
	if err != nil {
		log.Fatalln("Error:", err)
	}
}

func cls(c io.Closer) { _ = c.Close() }

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
	if errors.Is(content.ErrNotFound, err) || os.IsNotExist(err) {
		log.Println("[Err] Not found:", err)
		handleNotFound(c)
		return true
	}
	return false
}
