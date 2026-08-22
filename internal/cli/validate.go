package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/charleszardd/daegsa/internal/config"
	"github.com/charleszardd/daegsa/internal/plan"
	"github.com/charleszardd/daegsa/internal/safety"
	"github.com/spf13/cobra"
)

func newValidateCmd() *cobra.Command {
	var flags flagValues

	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate configuration syntax, environment placeholders, and safety preflight",
		Long:  "Validate configuration syntax, environment placeholders, and safety preflight without sending test traffic.",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, _, p, err := loadAndPreflightConfig(cmd.Context(), &flags)
			if err != nil {
				return err
			}

			fmt.Fprint(cmd.OutOrStdout(), plan.FormatPlanSummary(p))
			fmt.Fprintln(cmd.OutOrStdout(), "Configuration and safety preflight validation PASSED.")
			return nil
		},
	}

	addCommonFlags(cmd.Flags(), &flags)
	return cmd
}

func loadAndPreflightConfig(ctx context.Context, flags *flagValues) (*config.Config, *safety.PreflightResult, *plan.Plan, error) {
	var cfg *config.Config

	if flags.configFile != "" {
		rawBytes, err := os.ReadFile(flags.configFile)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("%w: failed to read config file %q: %w", config.ErrConfigValidation, flags.configFile, err)
		}

		expandedBytes, err := config.ExpandEnv(rawBytes, nil)
		if err != nil {
			return nil, nil, nil, err
		}

		parsed, err := config.ParseAndValidateYAML(expandedBytes)
		if err != nil {
			return nil, nil, nil, err
		}
		cfg = parsed
	} else {
		if flags.url == "" {
			return nil, nil, nil, fmt.Errorf("%w: either --config or --url must be provided", config.ErrConfigValidation)
		}
		cfg = &config.Config{
			SchemaVersion: config.ExpectedSchemaVersion,
			Name:          "cli-execution",
		}
	}

	// Apply CLI overrides
	cliFlags := flags.toCLIFlags()
	if err := config.ApplyCLIOverrides(cfg, cliFlags); err != nil {
		return nil, nil, nil, err
	}

	// Safety Preflight
	engine := safety.NewPreflightEngine()
	safetyFlags := safety.SafetyFlags{
		AllowDestructive: flags.allowDestructive,
		NonInteractive:   flags.nonInteractive,
	}
	preflightResult, err := engine.Check(ctx, cfg, safetyFlags)
	if err != nil {
		return nil, nil, nil, err
	}

	// Build immutable execution plan
	p, err := plan.BuildPlan(cfg, preflightResult)
	if err != nil {
		return nil, nil, nil, err
	}

	return cfg, preflightResult, p, nil
}
