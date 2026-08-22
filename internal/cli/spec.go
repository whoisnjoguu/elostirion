package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/whoisnjoguu/elostirion/pkg/spec"
)

// specCmd groups spec authoring and introspection subcommands
var specCmd = &cobra.Command{
	Use:   "spec",
	Short: "Author and inspect the fleet spec.",
}

var specInitCmd = &cobra.Command{
	Use:   "init [path]",
	Short: "Write a starter fleet-spec.yaml.",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := "fleet-spec.yaml"
		if len(args) == 1 {
			path = args[0]
		}
		if _, err := os.Stat(path); err == nil {
			return failure("%s already exists", path)
		}
		if err := os.WriteFile(path, []byte(spec.DefaultTemplate), 0o644); err != nil {
			return failure("write %s: %v", path, err)
		}
		fmt.Fprintf(os.Stdout, "wrote %s\n", path)
		return nil
	},
}

var specValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate the spec named by --spec.",
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := loadSpec()
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "ok: %d rule(s)\n", len(s.Rules))
		return nil
	},
}

var specFmtCmd = &cobra.Command{
	Use:   "fmt",
	Short: "Print the spec named by --spec in canonical form.",
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := loadSpec()
		if err != nil {
			return err
		}
		out, err := spec.Marshal(s)
		if err != nil {
			return failure("marshal spec: %v", err)
		}
		_, _ = os.Stdout.Write(out)
		return nil
	},
}

func init() {
	specCmd.AddCommand(specInitCmd, specValidateCmd, specFmtCmd)
	rootCmd.AddCommand(specCmd)
}
