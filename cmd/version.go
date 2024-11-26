package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	_ "embed"
)

func init() {
	rootCmd.AddCommand(versionCmd)
	versionCmd.Flags().String("short", "", "")
}

//go:embed version
var version []byte

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of Netirk CLI",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(string(version))
	},
}
