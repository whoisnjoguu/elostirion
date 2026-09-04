package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	// Blank imports register the built-in scanners via their init functions.
	_ "github.com/whoisnjoguu/elostirion/pkg/scan/dockerfile"
	_ "github.com/whoisnjoguu/elostirion/pkg/scan/gomod"
	_ "github.com/whoisnjoguu/elostirion/pkg/scan/python"
)

// Global flag values shared by subcommands.
var (
	specFlag   string
	formatFlag string
	failOn     string
	languages  []string
	dryRun     bool
	verbose    bool
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "elo",
	Short: "Manage a fleet of repositories against a declarative spec.",
	Long: `elostirion reads a fleet spec describing the desired state of every
repository; base image, language version, pipeline shape, Terraform
module versions, required files, env contract .e.t.c and reports or fixes
drift from that state. It does not require a server or a cluster.`,
	SilenceUsage:      true,
	SilenceErrors:     true,
	PersistentPreRunE: validateLanguages,
}

// Execute runs the root command. Exit codes follow the tool contract:
// 0 conformant, 1 findings or violations, 2 tool error.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		if ec, ok := err.(exitError); ok {
			if ec.code != 0 && ec.msg != "" {
				fmt.Fprintln(os.Stderr, "elo:", ec.Error())
			}
			os.Exit(ec.code)
		}
		fmt.Fprintln(os.Stderr, "elo:", err)
		os.Exit(2)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&specFlag, "spec", "s", "", "fleet spec: local path, git:: URL, or http(s) URL")
	rootCmd.PersistentFlags().StringVarP(&formatFlag, "format", "f", "text", "output format: text, json, junit, sarif")
	rootCmd.PersistentFlags().StringVar(&failOn, "fail-on", "error", "minimum severity that fails: error, drift, warn")
	rootCmd.PersistentFlags().StringSliceVarP(&languages, "language", "l", nil, "languages to scan (for example go, py); repeatable or comma-separated. Default: all")
	rootCmd.PersistentFlags().BoolVarP(&dryRun, "dry-run", "d", false, "report changes without making them")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose output")
}
