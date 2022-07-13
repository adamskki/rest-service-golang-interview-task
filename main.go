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

// Functions which is responsible for converting plain response from RANDOMORD API to array of integers values
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

// Adding required query params to RANDOMORG API url
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

	return math.Round(math.Sqrt(standardDeviation/float64(len(numbers)))*100) / 100
}

func isTimeout(err error) bool {
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

	// Setting 5 seconds timeout on http request
	httpClient := &http.Client{
		Timeout: time.Second * 5,
	}

	// Sending http requests with cancellation context
	response, err := httpClient.Do(req.WithContext(ctx))

	if err != nil {
		if isTimeout(err) {
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

	// Reading response body
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

	// Returning results
	defaultChannel <- StandardDeviation{calculateStandardDeviation(numbers), numbers}
}

func randomMeanHandler(c *gin.Context) {
	randomMeanQueryParams := RandomMeanQueryParams{}
	// Query Params validation
	if err := c.ShouldBindQuery(&randomMeanQueryParams); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest,
			gin.H{
				"error": "QUERY_PARAMS_ERROR",
				"message": "Invalid query params. Requests and length query parameters are required. " +
					"Requests must be an integer value in [1,1000] interval " +
					"and length must be integer value in [1,10000] interval"})
		return
	}

	// Preparing URL for external API call
	baseUrl, _ := url.Parse(RandomIntegerServiceUrl)
	addRequiredQueryParamsToUrl(baseUrl, strconv.Itoa(randomMeanQueryParams.Length))

	// Channels which are used to communicate with child goroutines
	defaultChannel := make(chan StandardDeviation)
	errorChannel := make(chan error, randomMeanQueryParams.Requests)

	// Context with cancel which is used to interrupt http requests send by child goroutines
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Variables which holds results from child goroutines
	var standardDeviations []StandardDeviation
	var allNumbers []int

	// Creating goroutines in order to make concurrent http requests to RANDOMORG API
	for i := 0; i < randomMeanQueryParams.Requests; i++ {
		go getStandardDeviationFromRandomNumbers(ctx, baseUrl, defaultChannel, errorChannel)
	}

	// Gathering results from child goroutines
	for i := 0; i < randomMeanQueryParams.Requests; i++ {
		select {
		// When an error occurs in one child goroutine, processing in others is also interrupted
		case err := <-errorChannel:
			log.Print("Errors occurs in worker goroutine, cancelling other requests")
			cancel()
			c.AbortWithStatusJSON(http.StatusInternalServerError,
				gin.H{
					"error":   "RANDOM_ORG_ACCESS_ERROR",
					"message": err.Error()})
			return
		//	Gathering standard deviation from child goroutines
		case stddev := <-defaultChannel:
			standardDeviations = append(standardDeviations, stddev)
			allNumbers = append(allNumbers, stddev.Data...)
		//	Case when client cancel request
		case <-c.Done():
			log.Print("Client canceled request")
			cancel()
		}

	}

	// Calculating and adding last standard deviation from all random numbers
	standardDeviations = append(
		standardDeviations,
		StandardDeviation{calculateStandardDeviation(allNumbers), allNumbers},
	)

	// Returning final result
	c.JSON(http.StatusOK, standardDeviations)
}

func main() {
	router := gin.Default()
	router.GET("/random/mean", randomMeanHandler)
	router.Run(":8090")
}
