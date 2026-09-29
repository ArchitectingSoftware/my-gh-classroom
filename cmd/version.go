package cmd

import (
	"fmt"
	"io"
	"runtime"
	"runtime/debug"

	"github.com/spf13/cobra"
)

// version is set at build time by the Makefile:
//
//	go build -ldflags "-X github.com/ArchitectingSoftware/my-gh-classroom/cmd.version=v1.0.0"
//
// Builds without it report the module version when installed with
// `go install ...@vX.Y.Z`, or "dev" otherwise.
var version = ""

// buildVersion returns the version string and any VCS details Go embedded
// in the binary (commit, commit time, and whether the tree was modified).
func buildVersion() (v, commit, when string, modified bool) {
	v = version
	info, ok := debug.ReadBuildInfo()
	if ok {
		if v == "" && info.Main.Version != "" && info.Main.Version != "(devel)" {
			v = info.Main.Version
		}
		for _, s := range info.Settings {
			switch s.Key {
			case "vcs.revision":
				commit = s.Value
				if len(commit) > 12 {
					commit = commit[:12]
				}
			case "vcs.time":
				when = s.Value
			case "vcs.modified":
				modified = s.Value == "true"
			}
		}
	}
	if v == "" {
		v = "dev"
	}
	return v, commit, when, modified
}

func printVersion(w io.Writer) {
	v, commit, when, modified := buildVersion()
	fmt.Fprintf(w, "mgc %s\n", v)
	if commit != "" {
		if modified {
			commit += " (modified)"
		}
		fmt.Fprintf(w, "  commit:  %s\n", commit)
	}
	if when != "" {
		fmt.Fprintf(w, "  date:    %s\n", when)
	}
	fmt.Fprintf(w, "  go:      %s %s/%s\n", runtime.Version(), runtime.GOOS, runtime.GOARCH)
}

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show the mgc version and build details",
		Args:  cobra.NoArgs,
		Run:   func(c *cobra.Command, _ []string) { printVersion(c.OutOrStdout()) },
	}
}
