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
		Use:   "validate [flags]",
		Short: "Validate configuration syntax, environment placeholders, and safety preflight",
		Long: `Validate YAML test configuration syntax, resolve environment placeholders (${VAR}),
and execute preflight safety checks (host allowlists, method safeguards, DNS resolution)
without sending any test traffic.`,
		Example: `  # 1. Validate a YAML configuration file:
  daegsa validate --config examples/open-api-capacity.yaml

  # 2. Validate with CLI parameter overrides:
  daegsa validate --config examples/open-api-capacity.yaml --rate 200 --duration 1m

  # 3. Validate ad-hoc command line parameters:
  daegsa validate --url "https://api.example.com/items" --model open --rate 50 --duration 30s`,
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
			SchemaVersion: config.LegacySchemaVersion,
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
