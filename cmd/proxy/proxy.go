package main

import (
	"github.com/asmogo/nws/config"
	"github.com/asmogo/nws/gw"
	"golang.org/x/net/context"
)

func main() {
	// load the configuration
	// from the environment
	cfg, err := config.LoadConfig[config.ProxyConfig]()
	if err != nil {
		panic(err)
	}

	// create a new gw server
	// and start it
	proxy := gw.NewProxy(context.Background(), cfg)

	err = proxy.Start()
	if err != nil {
		panic(err)
	}
}
