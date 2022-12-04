package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"runtime/debug"

	"github.com/spf13/cobra"
)

// versionCmd is gom version.
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "returns the version of the program",
	Long:  `returns the version of the program`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return versionImpl.version(cmd.Context(), args, cmd.OutOrStdout(), cmd.OutOrStderr())
	},
}

type versionImplConfig struct {
}

var versionImpl versionImplConfig

var (
	ErrReadingBuildInfo = errors.New("error reading build info")
)

func (r *versionImplConfig) version(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return ErrReadingBuildInfo
	}

	fmt.Println(bi.Main.Version)

	return nil
}
