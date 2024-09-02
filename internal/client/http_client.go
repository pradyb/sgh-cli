package client

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httputil"
	"os"

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

	res, err := c.Client.Do(req)

	rateLimit := res.Header.Get("X-RateLimit-Limit")
	rateRemaining := res.Header.Get("X-RateLimit-Remaining")
	rateUsed := res.Header.Get("X-RateLimit-Used")
	rateReset := res.Header.Get("X-RateLimit-Reset")
	rateResource := res.Header.Get("X-RateLimit-Resource")

	logger.Flog.Info().Msgf("Invoked %s request to %s received status %d", req.Method, req.URL.String(), res.StatusCode)
	logger.Flog.Info().Str("rateLimit", rateLimit).Str("rateRemaining", rateRemaining).Str("rateUsed", rateUsed).Str("rateResource", rateResource).Str("rateReset", rateReset).Msgf("Rate Limit: %s, Remaining: %s, Used: %s, Reset: %s, Resource: %s", rateLimit, rateRemaining, rateUsed, rateReset, rateResource)

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
	Client *githubv4.Client
}

func (c *GraphqlClient) Query(query interface{}, variables map[string]interface{}) error {
	err := c.Client.Query(context.Background(), query, variables)
	if err != nil {
		fmt.Println(err)
		logger.Glog.Error().Err(err).Msg("Error in executing the query")
	}
	return err
}
