package cmd

import (
	"log"

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
	if err := RootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func init() {
	RootCmd.AddCommand(playCmd)
	RootCmd.AddCommand(versionCmd)
}
