package main

import (
	"github.com/asmogo/nws/config"
	"github.com/asmogo/nws/exit"

	"golang.org/x/net/context"
)

func main() {
	// load the configuration
	// from the environment
	cfg, err := config.LoadConfig[config.ExitConfig]()
	if err != nil {
		panic(err)
	}

	// create a new gw server
	// and start it
	ctx := context.Background()
	exitNode := exit.NewExit(ctx, cfg)
	err = exitNode.SetSubscriptions(ctx)
	if err != nil {
		panic(err)
	}
}
