package main

import (
	"github.com/asmogo/nws/config"
	"github.com/asmogo/nws/exit"
	"github.com/nbd-wtf/go-nostr"
	"github.com/spf13/cobra"
	"log/slog"
)

var httpsPort int32
var httpTarget string

const (
	generateKeyMessage = "Generated new private key. Please set your environment using the new key, otherwise your key will be lost."
	usagePort          = "set the https reverse proxy port"
	usageTarget        = "set https reverse proxy target (your local service)"
)

func main() {
	rootCmd := &cobra.Command{Use: "exit", Run: startExitNode}
	rootCmd.Flags().Int32VarP(&httpsPort, "port", "p", 0, usagePort)
	rootCmd.Flags().StringVarP(&httpTarget, "target", "t", "", usageTarget)
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
	cfg.HttpsPort = httpsPort
	cfg.HttpsTarget = httpTarget

	if cfg.NostrPrivateKey == "" {
		// generate new private key
		cfg.NostrPrivateKey = nostr.GeneratePrivateKey()
		slog.Warn(generateKeyMessage, "key", cfg.NostrPrivateKey)
	}
	// create a new gw server
	// and start it
	ctx := cmd.Context()
	exitNode := exit.NewExit(ctx, cfg)
	exitNode.ListenAndServe(ctx)
}
