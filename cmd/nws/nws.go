package main

import (
	"fmt"
	"log/slog"

	"github.com/asmogo/nws/config"
	"github.com/asmogo/nws/exit"
	"github.com/asmogo/nws/proxy"
	"github.com/spf13/cobra"
)

const (
	usagePort   = "set the https reverse proxy port"
	usageTarget = "set https reverse proxy target (your local service)"
)

func main() {
	rootCmd := &cobra.Command{Use: "nws"}
	exitCmd := &cobra.Command{Use: "exit", Run: startExitNode}
	var httpsPort int32
	var httpTarget string
	exitCmd.Flags().Int32VarP(&httpsPort, "port", "p", 0, usagePort)
	exitCmd.Flags().StringVarP(&httpTarget, "target", "t", "", usageTarget)
	entryCmd := &cobra.Command{Use: "entry", Run: startEntryNode}
	rootCmd.AddCommand(exitCmd)
	rootCmd.AddCommand(entryCmd)
	err := rootCmd.Execute()
	if err != nil {
		panic(err)
	}
}

// updateConfigFlag updates the configuration with the provided flags.
func updateConfigFlag(cmd *cobra.Command, cfg *config.ExitConfig) error {
	httpsPort, err := cmd.Flags().GetInt32("port")
	if err != nil {
		return fmt.Errorf("failed to get https port: %w", err)
	}
	httpTarget, err := cmd.Flags().GetString("target")
	if err != nil {
		return fmt.Errorf("failed to get http target: %w", err)
	}
	cfg.HttpsPort = httpsPort
	cfg.HttpsTarget = httpTarget
	return nil
}

func startExitNode(cmd *cobra.Command, _ []string) {
	slog.Info("Starting exit node")
	// load the configuration
	cfg, err := config.LoadConfig[config.ExitConfig]()
	if err != nil {
		panic(err)
	}
	if len(cfg.NostrRelays) == 0 {
		slog.Info("No relays provided, using default relays")
		cfg.NostrRelays = config.DefaultRelays
	}
	err = updateConfigFlag(cmd, cfg)
	if err != nil {
		panic(err)
	}
	ctx := cmd.Context()
	exitNode := exit.NewExit(ctx, cfg)
	exitNode.ListenAndServe(ctx)
}

func startEntryNode(cmd *cobra.Command, _ []string) {
	slog.Info("Starting entry node")
	cfg, err := config.LoadConfig[config.EntryConfig]()
	if err != nil {
		panic(err)
	}
	// create a new gw server
	socksProxy := proxy.New(cmd.Context(), cfg)
	err = socksProxy.Start()
	if err != nil {
		panic(err)
	}

}
