package cli

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

// Build metadata variables injected at build time via ldflags (§13, §15).
var (
	Version   = "v0.1.0-dev"
	Commit    = "unknown"
	BuildDate = "unknown"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print DAEGSA version and build metadata",
		Long:  "Print DAEGSA version, Git commit SHA, build date, Go runtime version, OS, and architecture.",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("daegsa version %s (commit: %s, built: %s, runtime: %s %s/%s)\n",
				Version, Commit, BuildDate, runtime.Version(), runtime.GOOS, runtime.GOARCH)
		},
	}
}
