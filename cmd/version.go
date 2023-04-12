package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(versionCmd)
	versionCmd.Flags().String("short", "", "")
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of Netirk CLI",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Netirk static network tester v0.1")
	},
}
