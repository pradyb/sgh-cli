package context

import (
	"net/http"
	"time"

	"github.com/prady-lab/sgh-cli/internal/client"
	"github.com/prady-lab/sgh-cli/internal/config"
)

type Context struct {
	Config      *config.Config
	HttpClient  *client.HttpClient
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
	ctx.HttpClient = &client.HttpClient{Client: http.Client{Timeout: time.Duration(30) * time.Second}}

	return &ctx, nil
}

func (c *Context) SetVerbose(verbose bool) {
	c.Verbose = verbose
	c.HttpClient.Verbose = verbose
}

func (c *Context) SetLogResponse(logResponse bool) {
	c.HttpClient.LogResponse = logResponse
}
