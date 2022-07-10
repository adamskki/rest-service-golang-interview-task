package main

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

func randomMeanHandler(c *gin.Context) {
	requests := c.DefaultQuery("requests", "1")
	length := c.DefaultQuery("length", "5")
	c.String(http.StatusOK, "Requests: %s , length: %s", requests, length)
}

func main() {
	router := gin.Default()
	router.GET("/random/mean", randomMeanHandler)
	router.Run("localhost:8090")
}
