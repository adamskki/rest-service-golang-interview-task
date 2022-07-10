package main

import (
	"github.com/gin-gonic/gin"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const RandomIntegerServiceUrl = "https://www.random.org/integers/"

type RandomMeanQueryParams struct {
	Requests uint `form:"requests" binding:"required,gte=0,lte=1000"`
	Length   uint `form:"length" binding:"required"`
}

func convertPlainResponseToIntArray(responseBody []byte) ([]int, error) {
	var integersArray []int
	for _, num := range strings.Fields(string(responseBody)) {
		value, err := strconv.Atoi(num)
		if err != nil {
			return nil, err
		}
		integersArray = append(integersArray, value)
	}
	return integersArray, nil
}

func addRequiredQueryParamsToUrl(baseUrl *url.URL, numLength string) {
	queryParams := url.Values{}
	queryParams.Add("num", numLength)
	queryParams.Add("min", "1")
	queryParams.Add("max", "1000")
	queryParams.Add("col", "1")
	queryParams.Add("base", "10")
	queryParams.Add("format", "plain")

	baseUrl.RawQuery = queryParams.Encode()
}

func calculateMean(numbers []int) float64 {
	var sum int

	if len(numbers) == 0 {
		return 0
	}

	for _, num := range numbers {
		sum += num
	}
	return float64(sum) / float64(len(numbers))
}

func calculateStandardDeviation(numbers []int) float64 {
	var standardDeviation float64
	mean := calculateMean(numbers)

	for _, num := range numbers {
		standardDeviation += math.Pow(float64(num)-mean, 2)
	}

	return math.Sqrt(standardDeviation / float64(len(numbers)))
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

	httpClient := &http.Client{
		Timeout: time.Second * 10,
	}

	baseUrl, _ := url.Parse(RandomIntegerServiceUrl)

	addRequiredQueryParamsToUrl(baseUrl, strconv.Itoa(int(randomMeanQueryParams.Length)))

	response, err := httpClient.Get(baseUrl.String())
	if err != nil {
		log.Fatal(err)
		return
	}
	if response.StatusCode != 200 {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"Error": "Random Org is not available",
		})
	}

	//defer response.Body.Close()
	body, _ := ioutil.ReadAll(response.Body)

	numbers, _ := convertPlainResponseToIntArray(body)

	standardDeviation := calculateStandardDeviation(numbers)

	c.JSON(http.StatusOK, gin.H{
		"stddev": math.Floor(standardDeviation*100) / 100,
		"data":   numbers,
	})
}

func main() {
	router := gin.Default()
	router.GET("/random/mean", randomMeanHandler)
	router.Run("localhost:8090")
}
