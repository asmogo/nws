package main

import (
	"github.com/asmogo/nws/config"
	"github.com/asmogo/nws/exit"
	"github.com/spf13/cobra"
)

var httpsPort int32
var httpTarget string

const (
	usagePort   = "set the https reverse proxy port"
	usageTarget = "set https reverse proxy target (your local service)"
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

// updateConfigFlag updates the configuration with the provided flags.
func updateConfigFlag(cfg *config.ExitConfig) {
	cfg.HttpsPort = httpsPort
	cfg.HttpsTarget = httpTarget
}

func startExitNode(cmd *cobra.Command, args []string) {
	// load the configuration
	// from the environment
	cfg, err := config.LoadConfig[config.ExitConfig]()
	if err != nil {
		panic(err)
	}
	updateConfigFlag(cfg)
	ctx := cmd.Context()
	exitNode := exit.NewExit(ctx, cfg)
	exitNode.ListenAndServe(ctx)
}
