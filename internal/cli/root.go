package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/charleszardd/daegsa/internal/core"
	"github.com/spf13/cobra"
)

// NewRootCmd creates the root Cobra command for DAEGSA (§3, §5, §15).
func NewRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "daegsa",
		Short: "DAEGSA - REST API Load, Capacity, and Rate-Limit Testing CLI",
		Long: `DAEGSA is a portable CLI for repeatable REST API load, capacity, stress,
spike, soak, and rate-limit testing with explicit open and closed workload models.`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	rootCmd.AddCommand(newRunCmd())
	rootCmd.AddCommand(newValidateCmd())
	rootCmd.AddCommand(newVersionCmd())

	return rootCmd
}

// Execute executes the CLI with command-line arguments and returns the process exit code (§10).
func Execute() core.ExitCode {
	return ExecuteContext(context.Background(), os.Args[1:])
}

// ExecuteContext runs the CLI commands with the provided context and arguments.
func ExecuteContext(ctx context.Context, args []string) core.ExitCode {
	rootCmd := NewRootCmd()
	rootCmd.SetArgs(args)

	err := rootCmd.ExecuteContext(ctx)
	if err != nil {
		code := DetermineExitCode(err)
		fmt.Fprintln(os.Stderr, FormatSingleLineSummary(err, code))
		return code
	}

	return core.ExitCodeSuccess
}
