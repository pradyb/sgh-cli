package client

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httputil"
	"os"
	"strconv"
	"time"

	logger "github.com/prady-lab/sgh-cli/utils"
	"github.com/shurcooL/githubv4"
)

type HttpClient struct {
	Client      http.Client
	Verbose     bool
	LogResponse bool
}

func (c *HttpClient) Send(req *http.Request) (*http.Response, error) {
	req.Header.Add("Authorization", fmt.Sprintf("token %s", os.Getenv("GITHUB_TOKEN")))
	req.Header.Add("Content-Type", "application/json")

	if c.Verbose {
		reqDump, err := httputil.DumpRequestOut(req, true)
		if err != nil {
			logger.Glog.Error().Err(err).Msg("Error in print the request")
		}
		fmt.Printf("REQUEST:\n%s", string(reqDump))
	}

	start := time.Now()
	res, err := c.Client.Do(req)
	elapsed := time.Since(start).Milliseconds()

	rateLimit := res.Header.Get("X-RateLimit-Limit")
	rateRemaining := res.Header.Get("X-RateLimit-Remaining")
	rateUsed := res.Header.Get("X-RateLimit-Used")
	rateResetInt, _ := strconv.ParseInt(res.Header.Get("X-RateLimit-Reset"), 10, 64)
	rateReset := time.Unix(rateResetInt, 0).String()
	rateResource := res.Header.Get("X-RateLimit-Resource")

	logger.Flog.Info().Str("url", req.URL.String()).Str("method", req.Method).Int("statusCode", res.StatusCode).Int("timeTakenInMs", int(elapsed)).Msgf("API details")
	logger.Flog.Info().Str("rateLimit", rateLimit).Str("rateRemaining", rateRemaining).Str("rateUsed", rateUsed).Str("rateResource", rateResource).Str("rateReset", rateReset).Msgf("Rate limit details")

	if c.LogResponse {
		respDump, err := httputil.DumpResponse(res, true)
		if err != nil {
			logger.Glog.Error().Err(err).Msg("Error in print the response")
		}

		fmt.Printf("RESPONSE:\n%s", string(respDump))
	}

	return res, err
}

type GraphqlClient struct {
	Client  *githubv4.Client
	Verbose bool
}

func (c *GraphqlClient) Query(query interface{}, variables map[string]interface{}) error {
	if c.Verbose {
		logger.Flog.Info().Msgf("Executing the query  %s", query)
		logger.Flog.Info().Msgf("Executing the query with the variables %s", variables)
	}
	err := c.Client.Query(context.Background(), query, variables)
	if err != nil {
		logger.Glog.Error().Err(err).Msg("Error in executing the query")
	}
	return err
}
