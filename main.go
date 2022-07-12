package main

import (
	"context"
	"errors"
	"fmt"
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

func isTimeoutError(err error) bool {
	e, ok := err.(net.Error)
	return ok && e.Timeout()
}

func getNumbers(ctx context.Context, baseUrl *url.URL, defaultChannel chan StandardDeviation, errorChannel chan error) {
	req, err := http.NewRequest("GET", baseUrl.String(), nil)

	if err != nil {
		fmt.Println("Creating requests error", err)
		errorChannel <- err
		return
	}

	httpClient := &http.Client{
		Timeout: time.Second * 5,
	}

	response, err := httpClient.Do(req.WithContext(ctx))

	if err != nil {
		if errors.Is(err, context.Canceled) {
			fmt.Println("REQUEST SUCCESSFUL CANCELED")
		} else if isTimeoutError(err) {
			errorChannel <- errors.New("RandomORG request timeout")
		} else {
			fmt.Printf("error making request: %v\n", err)
			errorChannel <- errors.New("RandomORG request timeout")
		}
		return
	}

	if response.StatusCode != 200 {
		errorChannel <- errors.New("Service RandomORG is not available")
		fmt.Println("Server is not available!!!")
		return
	}

	//response, err := httpClient.Get(baseUrl.String())
	//if err != nil {
	//	errorChannel <- err
	//	log.Fatal(err)
	//}

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
	errorChannel := make(chan error, randomMeanQueryParams.Requests)

	ctx := context.Background()

	ctx, cancel := context.WithCancel(ctx)

	defer cancel()

	for i := 0; i < randomMeanQueryParams.Requests; i++ {
		go getNumbers(ctx, baseUrl, defaultChannel, errorChannel)
	}

	var standardDeviations []StandardDeviation
	var allNumbers []int

	for i := 0; i < randomMeanQueryParams.Requests; i++ {
		select {
		case err := <-errorChannel:
			fmt.Println("Errors occurs", err)
			cancel()
			c.JSON(http.StatusInternalServerError, err)
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
