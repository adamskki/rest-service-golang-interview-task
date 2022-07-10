package main

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

type RandomMeanQueryParams struct {
	Requests uint `form:"requests" binding:"required,gte=0,lte=1000"`
	Length   uint `form:"length" binding:"required,gte=0,lte=10"`
}

func randomMeanHandler(c *gin.Context) {
	randomMeanQueryParams := RandomMeanQueryParams{}
	if err := c.ShouldBindQuery(&randomMeanQueryParams); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest,
			gin.H{
				"error":   "QUERY_PARAMS_ERROR",
				"message": "Invalid query params. Please check your inputs"})
		return
	}
	c.String(http.StatusOK, "Requests Done!")
}

func main() {
	router := gin.Default()
	router.GET("/random/mean", randomMeanHandler)
	router.Run("localhost:8090")
}
