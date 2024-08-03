package context

import (
	"net/http"
	"time"

	"github.com/prady-lab/sgh-cli/internal/client"
	"github.com/prady-lab/sgh-cli/internal/config"
)

type Context struct {
	Config     *config.Config
	HttpClient *client.HttpClient
}

func Init() (*Context, error) {
	var ctx Context

	config, err := config.Init()
	if err != nil {
		return nil, err
	}

	ctx.Config = config
	ctx.HttpClient = &client.HttpClient{Client: http.Client{Timeout: time.Duration(30) * time.Second}, Verbose: config.Verbose}

	return &ctx, nil
}
