package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// colorPalette holds ANSI escape codes or empty strings if colors are disabled.
type colorPalette struct {
	reset   string
	bold    string
	dim     string
	cyan    string
	green   string
	yellow  string
	magenta string
	blue    string
	red     string
}

func getPalette(w io.Writer) colorPalette {
	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return colorPalette{}
	}
	return colorPalette{
		reset:   "\033[0m",
		bold:    "\033[1m",
		dim:     "\033[90m",
		cyan:    "\033[36m",
		green:   "\033[32m",
		yellow:  "\033[33m",
		magenta: "\033[35m",
		blue:    "\033[34m",
		red:     "\033[31m",
	}
}

// FormatRootHelp returns a colorful, beginner-friendly help page for the root daegsa command.
func FormatRootHelp(cmd *cobra.Command) string {
	c := getPalette(cmd.OutOrStdout())
	var sb strings.Builder

	// 1. Banner
	sb.WriteString(c.cyan + c.bold)
	sb.WriteString("  ____    _    _____ ____ ____    _    \n")
	sb.WriteString(" |  _ \\  / \\  | ____/ ___/ ___|  / \\   \n")
	sb.WriteString(" | | | |/ _ \\ |  _|| |  _\\___ \\ / _ \\  \n")
	sb.WriteString(" | |_| / ___ \\| |__| |_| |___) / ___ \\ \n")
	sb.WriteString(" |____/_/   \\_\\_____\\____|____/_/   \\_\\\n")
	sb.WriteString(c.reset)

	sb.WriteString(fmt.Sprintf("\n %s%sDAEGSA%s - REST API Load, Capacity, and Rate-Limit Testing CLI\n", c.bold, c.cyan, c.reset))
	sb.WriteString(fmt.Sprintf(" %s(대/Daega: Cost/Price + Dagsa: Traffic Influx/Surge)%s\n\n", c.dim, c.reset))

	// 2. Usage
	sb.WriteString(fmt.Sprintf("%s%sUSAGE:%s\n", c.bold, c.yellow, c.reset))
	sb.WriteString(fmt.Sprintf("  %sdaegsa%s %s[command]%s %s[flags]%s\n\n", c.green, c.reset, c.cyan, c.reset, c.dim, c.reset))

	// 3. Command Groups
	sb.WriteString(fmt.Sprintf("%s%sCORE TESTING COMMANDS:%s\n", c.bold, c.yellow, c.reset))
	sb.WriteString(fmt.Sprintf("  %s%-12s%s %sExecute load, capacity, or rate-limit test against a target API%s\n", c.green+c.bold, "run", c.reset, c.reset, c.reset))
	sb.WriteString(fmt.Sprintf("  %s%-12s%s %sValidate configuration YAML, ${VAR} placeholders, and safety rules%s\n", c.green+c.bold, "validate", c.reset, c.reset, c.reset))

	sb.WriteString(fmt.Sprintf("\n%s%sDIAGNOSTICS & VERIFICATION:%s\n", c.bold, c.yellow, c.reset))
	sb.WriteString(fmt.Sprintf("  %s%-12s%s %sRun local host diagnostics (clock precision, DNS, TLS, socket limits)%s\n", c.green+c.bold, "doctor", c.reset, c.reset, c.reset))
	sb.WriteString(fmt.Sprintf("  %s%-12s%s %sRun automated in-process self-tests against an embedded target server%s\n", c.green+c.bold, "self-test", c.reset, c.reset, c.reset))

	sb.WriteString(fmt.Sprintf("\n%s%sANALYSIS & CI:%s\n", c.bold, c.yellow, c.reset))
	sb.WriteString(fmt.Sprintf("  %s%-12s%s %sCompare two test reports (baseline vs candidate) for regressions%s\n", c.green+c.bold, "compare", c.reset, c.reset, c.reset))

	sb.WriteString(fmt.Sprintf("\n%s%sUTILITIES:%s\n", c.bold, c.yellow, c.reset))
	sb.WriteString(fmt.Sprintf("  %s%-12s%s %sPrint DAEGSA version, Git commit, build date, and OS/arch metadata%s\n", c.green+c.bold, "version", c.reset, c.reset, c.reset))
	sb.WriteString(fmt.Sprintf("  %s%-12s%s %sHelp about any command%s\n", c.green+c.bold, "help", c.reset, c.reset, c.reset))

	// 4. Quickstart Examples
	sb.WriteString(fmt.Sprintf("\n%s%sQUICKSTART EXAMPLES FOR BEGINNERS:%s\n", c.bold, c.yellow, c.reset))

	sb.WriteString(fmt.Sprintf("  %s1. Check if your machine is ready for high-precision load testing:%s\n", c.dim, c.reset))
	sb.WriteString(fmt.Sprintf("     %sdaegsa doctor%s\n\n", c.cyan, c.reset))

	sb.WriteString(fmt.Sprintf("  %s2. Run automated self-tests to see DAEGSA in action immediately:%s\n", c.dim, c.reset))
	sb.WriteString(fmt.Sprintf("     %sdaegsa self-test%s\n\n", c.cyan, c.reset))

	sb.WriteString(fmt.Sprintf("  %s3. Quick ad-hoc test (Open arrival model: 50 req/s for 15s):%s\n", c.dim, c.reset))
	sb.WriteString(fmt.Sprintf("     %sdaegsa run --url \"https://api.example.com/items\" --model open --rate 50 --duration 15s%s\n\n", c.cyan, c.reset))

	sb.WriteString(fmt.Sprintf("  %s4. Concurrency test (Closed model: 10 Virtual Users with think time):%s\n", c.dim, c.reset))
	sb.WriteString(fmt.Sprintf("     %sdaegsa run --url \"https://api.example.com/items\" --model closed --users 10 --duration 30s%s\n\n", c.cyan, c.reset))

	sb.WriteString(fmt.Sprintf("  %s5. Run from a declarative YAML config and save a JSON report:%s\n", c.dim, c.reset))
	sb.WriteString(fmt.Sprintf("     %sdaegsa run --config examples/open-api-capacity.yaml --output-json report.json%s\n\n", c.cyan, c.reset))

	sb.WriteString(fmt.Sprintf("  %s6. Compare baseline vs candidate reports for performance regressions:%s\n", c.dim, c.reset))
	sb.WriteString(fmt.Sprintf("     %sdaegsa compare baseline.json candidate.json%s\n\n", c.cyan, c.reset))

	// 5. Flags & Guidance
	sb.WriteString(fmt.Sprintf("%s%sFLAGS:%s\n", c.bold, c.yellow, c.reset))
	sb.WriteString(fmt.Sprintf("  %s-h, --help%s   Show help for daegsa\n\n", c.cyan, c.reset))

	sb.WriteString(fmt.Sprintf("%s%sLEARN MORE:%s\n", c.bold, c.yellow, c.reset))
	sb.WriteString(fmt.Sprintf("  Use %sdaegsa [command] --help%s for detailed options and examples for any command.\n", c.cyan, c.reset))
	sb.WriteString(fmt.Sprintf("  Operator Guide: %sdocs/OPERATIONS.md%s | Safety Runbook: %sdocs/SAFETY_RUNBOOK.md%s\n", c.magenta, c.reset, c.magenta, c.reset))

	return sb.String()
}

// SetupColorfulHelp configures a command to display colorful, formatted help.
func SetupColorfulHelp(cmd *cobra.Command) {
	cmd.SetHelpFunc(func(c *cobra.Command, args []string) {
		if c.Parent() == nil {
			fmt.Fprint(c.OutOrStdout(), FormatRootHelp(c))
			return
		}
		// Subcommand help
		p := getPalette(c.OutOrStdout())
		var sb strings.Builder

		sb.WriteString(fmt.Sprintf("\n%s%s %s- %s%s\n\n", p.bold+p.cyan, c.CommandPath(), p.reset, c.Short, p.reset))

		if c.Long != "" {
			sb.WriteString(fmt.Sprintf("%sDESCRIPTION:%s\n  %s\n\n", p.bold+p.yellow, p.reset, strings.ReplaceAll(strings.TrimSpace(c.Long), "\n", "\n  ")))
		}

		sb.WriteString(fmt.Sprintf("%sUSAGE:%s\n  %s%s%s\n\n", p.bold+p.yellow, p.reset, p.green, c.UseLine(), p.reset))

		if c.Example != "" {
			sb.WriteString(fmt.Sprintf("%sEXAMPLES:%s\n", p.bold+p.yellow, p.reset))
			for _, line := range strings.Split(strings.TrimSpace(c.Example), "\n") {
				trimmed := strings.TrimSpace(line)
				if strings.HasPrefix(trimmed, "#") {
					sb.WriteString(fmt.Sprintf("  %s%s%s\n", p.dim, trimmed, p.reset))
				} else if trimmed != "" {
					sb.WriteString(fmt.Sprintf("    %s%s%s\n", p.cyan, trimmed, p.reset))
				} else {
					sb.WriteString("\n")
				}
			}
			sb.WriteString("\n")
		}

		if c.HasAvailableLocalFlags() {
			sb.WriteString(fmt.Sprintf("%sFLAGS:%s\n", p.bold+p.yellow, p.reset))
			sb.WriteString(c.LocalFlags().FlagUsages())
			sb.WriteString("\n")
		}

		if c.HasAvailableInheritedFlags() {
			sb.WriteString(fmt.Sprintf("%sGLOBAL FLAGS:%s\n", p.bold+p.yellow, p.reset))
			sb.WriteString(c.InheritedFlags().FlagUsages())
			sb.WriteString("\n")
		}

		fmt.Fprint(c.OutOrStdout(), sb.String())
	})
}
