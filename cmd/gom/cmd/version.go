package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"runtime/debug"
	"strconv"

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

func init() {
}

type Vcs struct {
	Vcs      string
	Revision string
	Time     string
	Modified bool
}

func (r *versionImplConfig) version(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return errors.New("error reading build info")
	}

	vcs, ok := fromBuildInfo(bi)
	if !ok {
		return errors.New("error parsing build info")
	}

	fmt.Println(vcs.Revision)

	return nil
}

func fromBuildInfo(info *debug.BuildInfo) (*Vcs, bool) {

	var vcs Vcs
	for _, s := range info.Settings {
		// Look out for vcs keys as defined at:
		// https://github.com/golang/go/blob/db19b42ca8771c25aa09e3747812f0229d44e75c/src/cmd/go/internal/load/pkg.go#L2450-L2458
		switch s.Key {
		case "vcs.revision":
			vcs.Revision = s.Value
		case "vcs.time":
			vcs.Time = s.Value
		case "vcs.modified":
			b, err := strconv.ParseBool(s.Value)
			if err != nil {
				return nil, false
			}
			vcs.Modified = b
		case "vcs":
			vcs.Vcs = s.Value
		}
	}

	// If the main "vcs" key is not set in BuildInfo,
	// discard all the other vcs keys and return nil.
	if vcs.Vcs == "" {
		return nil, false
	}

	return &vcs, true
}
