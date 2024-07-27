package main

import (
	"github.com/asmogo/nws/config"
	"github.com/asmogo/nws/proxy"
	"golang.org/x/net/context"
)

func main() {
	// load the configuration
	// from the environment
	cfg, err := config.LoadConfig[config.EntryConfig]()
	if err != nil {
		panic(err)
	}

	// create a new gw server
	// and start it
	socksProxy := proxy.New(context.Background(), cfg)

	err = socksProxy.Start()
	if err != nil {
		panic(err)
	}
}
