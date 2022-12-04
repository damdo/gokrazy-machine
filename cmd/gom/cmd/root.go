package cmd

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
)

var RootCmd = &cobra.Command{
	Use:           "gom",
	Short:         "a lightweight virtual/emulated machine to run and develop for gokrazy",
	Long:          `a lightweight virtual/emulated machine to run and develop for gokrazy`,
	SilenceErrors: true,
	SilenceUsage:  true,
}

func Execute() {
	ctx, cancel := signal.NotifyContext(context.Background(),
		syscall.SIGINT, syscall.SIGTERM)

	RootCmd.SetContext(ctx)

	if err := RootCmd.Execute(); err != nil {
		log.Fatal(err)
	}

	cancel()
}

func init() {
	RootCmd.AddCommand(playCmd)
	RootCmd.AddCommand(versionCmd)
}
