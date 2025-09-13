package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "netirk",
	Short: "Netirk is a comprehensive network testing and monitoring CLI tool",
	Long: `Netirk is a portable network utility for comprehensive network testing, monitoring, and analysis.

Features:
• Network connectivity testing (HTTP, HTTPS, TCP)
• Continuous monitoring with configurable intervals
• SSL certificate monitoring and expiration tracking
• Performance analysis with trends and anomaly detection
• Multiple alert mechanisms (webhooks, email)
• Flexible output formats (JSON, CSV, HTML, Prometheus)
• Configuration validation and example generation

Commands:
  check     Test network connectivity to targets
  trace     Perform network timing analysis
  server    Run a lightweight HTTP server for testing
  monitor   Continuously monitor network targets
  analyze   Analyze historical monitoring data
  validate  Validate configuration files

Quick Start:
  # Generate example configuration
  netirk validate --generate-example config.yaml

  # Start monitoring
  netirk monitor --targets-file config.yaml --interval 30s

  # Analyze results
  netirk analyze --data-file monitoring.json --show-trends

Use "netirk [command] --help" for detailed information about each command.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	// Global output flag (used by trace, analyze, and as fallback for monitor)
	rootCmd.PersistentFlags().StringP("output", "o", "", "Output format: json, csv, html, prometheus (command-specific formats may vary)")
	
	// Global configuration options
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "Enable verbose output for debugging and detailed information")
	rootCmd.PersistentFlags().Bool("quiet", false, "Suppress non-essential output (opposite of verbose)")
	rootCmd.PersistentFlags().String("config", "", "Path to global configuration file (optional)")
}
