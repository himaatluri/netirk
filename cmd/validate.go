package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/himasagaratluri/netirk/cmd/helpers"
	"github.com/spf13/cobra"
)

var validateCmd = &cobra.Command{
	Use:   "validate [config-file]",
	Short: "Validate configuration files",
	Long: `Validate Netirk configuration files for syntax and content errors.
This command checks YAML syntax, validates target configurations, and provides
helpful suggestions for fixing any issues found.

Examples:
  netirk validate targets.yaml
  netirk validate monitoring-config.yaml
  netirk validate --generate-example example-config.yaml`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		generateExample, _ := cmd.Flags().GetBool("generate-example")
		verbose, _ := cmd.Flags().GetBool("verbose")
		
		// Check global flags and merge with local verbose flag
		globalVerbose, _ := rootCmd.PersistentFlags().GetBool("verbose")
		quiet, _ := rootCmd.PersistentFlags().GetBool("quiet")
		
		// Global verbose overrides local verbose
		if globalVerbose {
			verbose = true
		}
		
		// Suppress output in quiet mode
		if quiet && !verbose {
			// In quiet mode, only show errors and essential output
		}

		// Handle example generation
		if generateExample {
			filename := "netirk-config-example.yaml"
			if len(args) > 0 {
				filename = args[0]
			}
			
			if err := generateExampleConfig(filename); err != nil {
				fmt.Printf("Error generating example configuration: %v\n", err)
				os.Exit(1)
			}
			return
		}

		// Validate configuration file
		if len(args) == 0 {
			fmt.Println("Error: configuration file path is required")
			fmt.Println("Use 'netirk validate --help' for usage information")
			os.Exit(1)
		}

		configFile := args[0]
		if err := validateConfigurationFile(configFile, verbose); err != nil {
			fmt.Printf("Validation failed: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("✓ Configuration file '%s' is valid\n", configFile)
	},
}

// generateExampleConfig creates an example configuration file
func generateExampleConfig(filename string) error {
	// Check if file already exists
	if _, err := os.Stat(filename); err == nil {
		fmt.Printf("File '%s' already exists. Overwrite? (y/N): ", filename)
		var response string
		fmt.Scanln(&response)
		if response != "y" && response != "Y" {
			fmt.Println("Example generation cancelled.")
			return nil
		}
	}

	// Generate the example
	if err := helpers.GenerateConfigurationExample(filename); err != nil {
		return fmt.Errorf("failed to create example file: %w", err)
	}

	fmt.Printf("✓ Example configuration created: %s\n", filename)
	fmt.Println("\nThe example includes:")
	fmt.Println("  • HTTP and HTTPS target monitoring")
	fmt.Println("  • TCP connection monitoring")
	fmt.Println("  • SSL certificate monitoring")
	fmt.Println("  • Alert configuration with webhooks and email")
	fmt.Println("  • Custom headers and timeouts")
	fmt.Println("\nEdit the file to match your monitoring requirements.")
	
	return nil
}

// validateConfigurationFile validates a configuration file with detailed reporting
func validateConfigurationFile(filename string, verbose bool) error {
	fmt.Printf("Validating configuration file: %s\n", filename)

	// Check file extension
	ext := filepath.Ext(filename)
	if ext != ".yaml" && ext != ".yml" {
		fmt.Printf("Warning: File extension '%s' is not .yaml or .yml\n", ext)
	}

	// Perform validation
	err := helpers.ValidateConfigurationFile(filename)
	if err != nil {
		// Check if it's a ValidationError for better formatting
		if validationErr, ok := err.(helpers.ValidationError); ok {
			fmt.Printf("\nValidation Error: %s\n", validationErr.GetUserFriendlyMessage())
			if verbose {
				fmt.Printf("Error Code: %s\n", validationErr.ErrorCode)
			}
		}
		return err
	}

	// If validation passes, show summary
	if verbose {
		showConfigurationSummary(filename)
	}

	return nil
}

// showConfigurationSummary displays a summary of the validated configuration
func showConfigurationSummary(filename string) {
	targets, err := helpers.ParseEnhancedTargetFile(filename)
	if err != nil {
		return // Already validated, so this shouldn't happen
	}

	fmt.Printf("\nConfiguration Summary:\n")
	fmt.Printf("  Targets: %d\n", len(targets.Targets))

	// Count target types
	httpCount := 0
	httpsCount := 0
	tcpCount := 0
	sslEnabledCount := 0

	for _, target := range targets.Targets {
		if target.URL[:4] == "http" {
			if target.URL[:5] == "https" {
				httpsCount++
			} else {
				httpCount++
			}
		} else if target.URL[:3] == "tcp" {
			tcpCount++
		}

		if target.SSLCheck {
			sslEnabledCount++
		}
	}

	fmt.Printf("    HTTP targets: %d\n", httpCount)
	fmt.Printf("    HTTPS targets: %d\n", httpsCount)
	fmt.Printf("    TCP targets: %d\n", tcpCount)
	fmt.Printf("    SSL monitoring enabled: %d\n", sslEnabledCount)

	// Show timeout and interval ranges
	var minTimeout, maxTimeout, minInterval, maxInterval = targets.Targets[0].Timeout, targets.Targets[0].Timeout, targets.Targets[0].Interval, targets.Targets[0].Interval
	for _, target := range targets.Targets {
		if target.Timeout < minTimeout {
			minTimeout = target.Timeout
		}
		if target.Timeout > maxTimeout {
			maxTimeout = target.Timeout
		}
		if target.Interval < minInterval {
			minInterval = target.Interval
		}
		if target.Interval > maxInterval {
			maxInterval = target.Interval
		}
	}

	fmt.Printf("  Timeout range: %v - %v\n", minTimeout, maxTimeout)
	fmt.Printf("  Interval range: %v - %v\n", minInterval, maxInterval)
}

func init() {
	rootCmd.AddCommand(validateCmd)
	
	validateCmd.Flags().Bool("generate-example", false, "Generate an example configuration file")
	validateCmd.Flags().BoolP("verbose", "v", false, "Show detailed validation information")
}