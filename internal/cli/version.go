package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/whoisnjoguu/elostirion/internal/version"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version and exit.",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(version.String())
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
