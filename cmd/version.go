package cmd

import (
	"fmt"

	_ "embed"

	"github.com/himasagaratluri/netirk/cmd/helpers"
	"github.com/spf13/cobra"
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
		helpers.GreetBanner()
		fmt.Println(string(version))
	},
}
