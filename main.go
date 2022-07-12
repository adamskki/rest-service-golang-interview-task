package main

import (
	"context"
	"errors"
	"github.com/gin-gonic/gin"
	"io/ioutil"
	"log"
	"math"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const RandomIntegerServiceUrl = "https://www.random.org/integers/"

type RandomMeanQueryParams struct {
	Requests int `form:"requests" binding:"required,gte=0,lte=1000"`
	Length   int `form:"length" binding:"required,gte=0,lte=10000"`
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

func isTimeoutError(err error) bool {
	e, ok := err.(net.Error)
	return ok && e.Timeout()
}

func getStandardDeviationFromRandomNumbers(ctx context.Context, baseUrl *url.URL, defaultChannel chan StandardDeviation, errorChannel chan error) {
	req, err := http.NewRequest("GET", baseUrl.String(), nil)

	if err != nil {
		log.Print("Creating request error", err)
		errorChannel <- err
		return
	}

	httpClient := &http.Client{
		Timeout: time.Second * 5,
	}

	response, err := httpClient.Do(req.WithContext(ctx))

	if err != nil {
		if isTimeoutError(err) {
			errorChannel <- errors.New("server faced timeout error while accessing RANDOM.ORG service")
		} else {
			errorChannel <- errors.New("server faced error while accessing RANDOM.ORG service")
		}
		return
	}

	if response.StatusCode != 200 {
		errorChannel <- errors.New("server faced error error while accessing RANDOM.ORG service: " +
			http.StatusText(response.StatusCode))
		return
	}

	defer response.Body.Close()
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		errorChannel <- errors.New("server faced error while reading body response from RANDOM.ORG service")
		return
	}

	numbers, err := convertPlainResponseToIntArray(body)

	if err != nil {
		errorChannel <- errors.New("server faced error while parsing body response from RANDOM.ORG service")
		return
	}

	defaultChannel <- StandardDeviation{calculateStandardDeviation(numbers), numbers}
}

func randomMeanHandler(c *gin.Context) {
	randomMeanQueryParams := RandomMeanQueryParams{}
	if err := c.ShouldBindQuery(&randomMeanQueryParams); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest,
			gin.H{
				"error": "QUERY_PARAMS_ERROR",
				"message": "Invalid query params. Requests and length query parameters are required. " +
					"Requests must be an integer value in [1,1000] interval " +
					"and length must be integer value in [1,10000] interval"})
		return
	}

	baseUrl, _ := url.Parse(RandomIntegerServiceUrl)
	addRequiredQueryParamsToUrl(baseUrl, strconv.Itoa(int(randomMeanQueryParams.Length)))

	defaultChannel := make(chan StandardDeviation)
	errorChannel := make(chan error, randomMeanQueryParams.Requests)

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var standardDeviations []StandardDeviation
	var allNumbers []int

	for i := 0; i < randomMeanQueryParams.Requests; i++ {
		go getStandardDeviationFromRandomNumbers(ctx, baseUrl, defaultChannel, errorChannel)
	}

	for i := 0; i < randomMeanQueryParams.Requests; i++ {
		select {
		case err := <-errorChannel:
			log.Print("Errors occurs in worker goroutine, cancelling other requests")
			cancel()
			c.AbortWithStatusJSON(http.StatusInternalServerError,
				gin.H{
					"error":   "RANDOM_ORG_ACCESS_ERROR",
					"message": err.Error()})
			return
		case stddev := <-defaultChannel:
			standardDeviations = append(standardDeviations, stddev)
			allNumbers = append(allNumbers, stddev.Data...)
		case <-c.Done():
			log.Print("Client canceled request")
			cancel()
		}

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
