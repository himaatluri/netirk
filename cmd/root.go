package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "netirk",
	Short: "Netirk is a network testing cli",
	Long: `A portable network utility to check system reachability,
this utility can also be used to run a small http server when figuring out how to deploy a small
http server in a dynamic network landscape such as cloud platforms.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().StringP("output", "o", "", "Output to a special format like json.")
}
