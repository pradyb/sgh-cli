package client

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"os"

	logger "github.com/prady-lab/sgh-cli/utils"
)

type HttpClient struct {
	Client  http.Client
	Verbose bool
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

	if c.Verbose {
		respDump, err := httputil.DumpResponse(res, true)
		if err != nil {
			logger.Glog.Error().Err(err).Msg("Error in print the response")
		}

		fmt.Printf("RESPONSE:\n%s", string(respDump))
	}

	return res, err
}
