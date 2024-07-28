package main

import (
	"github.com/asmogo/nws/config"
	"github.com/asmogo/nws/exit"
	"github.com/spf13/cobra"
)

const (
	usagePort   = "set the https reverse proxy port"
	usageTarget = "set https reverse proxy target (your local service)"
)

func main() {

	var httpsPort int32
	var httpTarget string
	rootCmd := &cobra.Command{Use: "exit", Run: startExitNode}
	rootCmd.Flags().Int32VarP(&httpsPort, "port", "p", 0, usagePort)
	rootCmd.Flags().StringVarP(&httpTarget, "target", "t", "", usageTarget)
	err := rootCmd.Execute()
	if err != nil {
		panic(err)
	}
}

// updateConfigFlag updates the configuration with the provided flags.
func updateConfigFlag(cmd *cobra.Command, cfg *config.ExitConfig) error {

	httpsPort, err := cmd.Flags().GetInt32("port")
	if err != nil {
		return err
	}
	httpTarget, err := cmd.Flags().GetString("target")
	if err != nil {
		return err
	}
	cfg.HttpsPort = httpsPort
	cfg.HttpsTarget = httpTarget
	return nil
}

func startExitNode(cmd *cobra.Command, args []string) {
	// load the configuration
	// from the environment
	cfg, err := config.LoadConfig[config.ExitConfig]()
	if err != nil {
		panic(err)
	}
	updateConfigFlag(cmd, cfg)
	ctx := cmd.Context()
	exitNode := exit.NewExit(ctx, cfg)
	exitNode.ListenAndServe(ctx)
}
