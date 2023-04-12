package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

type TargetSystem struct {
	hostname string
	hostport int
}

var targetHost TargetSystem

var rootCmd = &cobra.Command{
	Use:   "netirk",
	Short: "Netirk is a network testing cli",
	Long:  `A quick test related to service availability`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
