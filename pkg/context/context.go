package context

import (
	"context"
	"net/http"
	"os"
	"time"

	"github.com/prady-lab/sgh-cli/internal/client"
	"github.com/prady-lab/sgh-cli/internal/config"

	"github.com/shurcooL/githubv4"
	"golang.org/x/oauth2"
)

type Context struct {
	Config        *config.Config
	HttpClient    *client.HttpClient
	GraphqlClient *client.GraphqlClient

	Verbose     bool
	LogResponse bool
}

func Init() (*Context, error) {
	var ctx Context

	config, err := config.Init()
	if err != nil {
		return nil, err
	}

	ctx.Config = config
	ctx.HttpClient = &client.HttpClient{Client: http.Client{Timeout: time.Duration(30) * time.Second, Transport: client.Interceptor{OriginalTransport: http.DefaultTransport}}}

	src := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: os.Getenv("GITHUB_TOKEN")},
	)

	defaultCtx := context.WithValue(context.Background(), oauth2.HTTPClient, &http.Client{Timeout: time.Duration(30) * time.Second, Transport: client.Interceptor{OriginalTransport: http.DefaultTransport}})
	httpClient := oauth2.NewClient(defaultCtx, src)

	gqlClient := githubv4.NewClient(httpClient)
	ctx.GraphqlClient = &client.GraphqlClient{Client: gqlClient}

	return &ctx, nil
}

func (c *Context) SetVerbose(verbose bool) {
	c.Verbose = verbose
	c.HttpClient.Verbose = verbose
	c.GraphqlClient.Verbose = verbose
}

func (c *Context) SetLogResponse(logResponse bool) {
	c.HttpClient.LogResponse = logResponse
}
