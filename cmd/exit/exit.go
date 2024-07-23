package main

import (
	"fmt"
	"github.com/asmogo/nws/config"
	"github.com/asmogo/nws/exit"
	"github.com/nbd-wtf/go-nostr"
	"github.com/spf13/cobra"
	"log/slog"
)

var httpsPort int32
var httpTarget string

const (
	generateKeyMessage          = "Generated new private key. Please update your configuration file with the new key, otherwise your key will be lost, once this application restarts."
	startingReverseProxyMessage = "starting exit node with https reverse proxy"
)

func main() {
	rootCmd := &cobra.Command{Use: "exit", Run: startExitNode}
	rootCmd.Flags().Int32VarP(&httpsPort, "port", "p", 0, "port for the https reverse proxy")
	rootCmd.Flags().StringVarP(&httpTarget, "target", "t", "", "target for the https reverse proxy (your local service)")
	err := rootCmd.Execute()
	if err != nil {
		panic(err)
	}
}
func startExitNode(cmd *cobra.Command, args []string) {

	// load the configuration
	// from the environment
	cfg, err := config.LoadConfig[config.ExitConfig]()
	if err != nil {
		panic(err)
	}
	if httpsPort != 0 {
		cfg.BackendHost = fmt.Sprintf(":%d", httpsPort)
	}
	if cfg.NostrPrivateKey == "" {
		// generate new private key
		cfg.NostrPrivateKey = nostr.GeneratePrivateKey()
		slog.Warn(generateKeyMessage, "key", cfg.NostrPrivateKey)
	}
	// create a new gw server
	// and start it
	ctx := cmd.Context()
	exitNode := exit.NewExit(ctx, cfg)
	if httpsPort != 0 {
		slog.Info(startingReverseProxyMessage, "port", httpsPort)
		go func() {
			err = exitNode.StartReverseProxy(httpTarget, httpsPort)
			if err != nil {
				panic(err)
			}
		}()

	}
	exitNode.ListenAndServe(ctx)
}
