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
	Requests int  `form:"requests" binding:"required,gte=0,lte=1000"`
	Length   uint `form:"length" binding:"required"`
}

type StandardDeviation struct {
	Stddev float64 `json:"stddev"`
	Data   []int   `json:"data"`
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

func getNumbers(baseUrl *url.URL, defaultChannel chan StandardDeviation, errorChannel chan error) {
	httpClient := &http.Client{
		Timeout: time.Second * 10,
	}

	response, err := httpClient.Get(baseUrl.String())
	if err != nil {
		log.Fatal(err)
		return
	}

	//defer response.Body.Close()
	body, _ := ioutil.ReadAll(response.Body)
	numbers, _ := convertPlainResponseToIntArray(body)

	defaultChannel <- StandardDeviation{calculateStandardDeviation(numbers), numbers}

	//if response.StatusCode != 200 {
	//	c.JSON(http.StatusServiceUnavailable, gin.H{
	//		"Error": "Random Org is not available",
	//	})
	//}
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

	baseUrl, _ := url.Parse(RandomIntegerServiceUrl)

	addRequiredQueryParamsToUrl(baseUrl, strconv.Itoa(int(randomMeanQueryParams.Length)))

	defaultChannel := make(chan StandardDeviation)
	errorChannel := make(chan error)

	for i := 0; i < randomMeanQueryParams.Requests; i++ {
		go getNumbers(baseUrl, defaultChannel, errorChannel)
	}

	var standardDeviations []StandardDeviation
	var allNumbers []int

	for i := 0; i < randomMeanQueryParams.Requests; i++ {
		stddev := <-defaultChannel
		standardDeviations = append(standardDeviations, stddev)
		allNumbers = append(allNumbers, stddev.Data...)
	}

	standardDeviations = append(
		standardDeviations,
		StandardDeviation{calculateStandardDeviation(allNumbers), allNumbers},
	)

	c.JSON(http.StatusOK, standardDeviations)
}

func main() {
	router := gin.Default()
	router.GET("/random/mean", randomMeanHandler)
	router.Run("localhost:8090")
}
