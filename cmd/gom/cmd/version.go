package cmd

import (
	"errors"
	"fmt"
	"runtime/debug"
	"strings"

	"github.com/spf13/cobra"
)

// versionCmd is gom version.
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "returns the version of the program",
	Long:  `returns the version of the program`,
	RunE: func(_ *cobra.Command, _ []string) error {
		return versionImpl.version()
	},
}

type versionImplConfig struct {
}

var versionImpl versionImplConfig

var (
	ErrReadingBuildInfo = errors.New("error reading build info")
)

func (r *versionImplConfig) version() error {
	fmt.Println(readVersion())

	return nil
}

func readVersionParts() (revision string, modified, ok bool) {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "", false, false
	}
	settings := make(map[string]string)
	for _, s := range info.Settings {
		settings[s.Key] = s.Value
	}
	// When built from a local VCS directory, we can use vcs.revision directly.
	if rev, ok := settings["vcs.revision"]; ok {
		return rev, settings["vcs.modified"] == "true", true
	}
	// When built as a Go module (not from a local VCS directory),
	// info.Main.Version is something like v0.0.0-20230107144322-7a5757f46310.
	v := info.Main.Version // for convenience
	if idx := strings.LastIndexByte(v, '-'); idx > -1 {
		return v[idx+1:], false, true
	}
	return "<BUG>", false, false
}

func readVersion() string {
	revision, modified, ok := readVersionParts()
	if !ok {
		return "<unknown>"
	}
	modifiedSuffix := ""
	if modified {
		modifiedSuffix = " (modified)"
	}

	return revision + modifiedSuffix
}
