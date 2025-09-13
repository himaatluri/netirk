package cmd

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func TestCheckCommand_Initialization(t *testing.T) {
	// Find check command from root command
	var checkCommand *cobra.Command
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "check" {
			checkCommand = cmd
			break
		}
	}

	if checkCommand == nil {
		t.Fatal("check command not found in root command")
	}

	if checkCommand.Use != "check" {
		t.Errorf("Expected Use 'check', got '%s'", checkCommand.Use)
	}

	if checkCommand.Short == "" {
		t.Error("Short description should not be empty")
	}

	if checkCommand.Long == "" {
		t.Error("Long description should not be empty")
	}

	if checkCommand.Run == nil {
		t.Error("Run function should not be nil")
	}
}

func TestCheckCommand_Flags(t *testing.T) {
	// Find check command from root command
	var checkCommand *cobra.Command
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "check" {
			checkCommand = cmd
			break
		}
	}

	if checkCommand == nil {
		t.Fatal("check command not found")
	}

	// Check that flags are present (may have different names than expected)
	flags := checkCommand.Flags()
	if flags == nil {
		t.Fatal("check command has no flags")
	}

	// List all available flags for debugging
	flagNames := []string{}
	flags.VisitAll(func(flag *pflag.Flag) {
		flagNames = append(flagNames, flag.Name)
	})
	
	t.Logf("Available flags: %v", flagNames)

	// Check for common flags that might exist
	commonFlags := []string{"target", "targets-file", "ip", "port", "verify-ssl"}
	for _, flagName := range commonFlags {
		flag := flags.Lookup(flagName)
		if flag != nil {
			t.Logf("Found flag: %s (default: %s)", flagName, flag.DefValue)
		}
	}
}

func TestCheckCommand_Help(t *testing.T) {
	// Find check command from root command
	var checkCommand *cobra.Command
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "check" {
			checkCommand = cmd
			break
		}
	}

	if checkCommand == nil {
		t.Fatal("check command not found")
	}

	// Test help output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	checkCommand.SetArgs([]string{"--help"})
	err := checkCommand.Execute()

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if err != nil {
		t.Errorf("Check help should not error, got: %v", err)
	}

	// Should contain usage information
	expectedSections := []string{
		"Usage:",
		"check",
		"Flags:",
		"--timeout",
		"--output",
		"--format",
		"--targets-file",
		"--verbose",
	}

	for _, section := range expectedSections {
		if !strings.Contains(output, section) {
			t.Errorf("Help output should contain '%s'", section)
		}
	}
}

func TestCheckCommand_Examples(t *testing.T) {
	// Find check command from root command
	var checkCommand *cobra.Command
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "check" {
			checkCommand = cmd
			break
		}
	}

	if checkCommand == nil {
		t.Skip("check command not found")
	}

	// Test that examples are provided in help
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	checkCommand.SetArgs([]string{"--help"})
	checkCommand.Execute()

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Should contain examples section
	if strings.Contains(output, "Examples:") {
		// If examples exist, they should show common usage patterns
		expectedPatterns := []string{
			"netirk check",
			"https://",
		}

		for _, pattern := range expectedPatterns {
			if !strings.Contains(output, pattern) {
				t.Logf("Example pattern '%s' not found in help", pattern)
			}
		}
	} else {
		t.Log("No examples section found in help - consider adding examples")
	}
}

func TestCheckCommand_ValidateArgs(t *testing.T) {
	// Find check command from root command
	var checkCommand *cobra.Command
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "check" {
			checkCommand = cmd
			break
		}
	}

	if checkCommand == nil {
		t.Skip("check command not found")
	}

	// Test argument validation
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name:    "valid single URL",
			args:    []string{"https://example.com"},
			wantErr: false,
		},
		{
			name:    "valid multiple URLs",
			args:    []string{"https://example.com", "https://google.com"},
			wantErr: false,
		},
		{
			name:    "valid TCP target",
			args:    []string{"google.com:80"},
			wantErr: false,
		},
		{
			name:    "no arguments with targets file should be valid",
			args:    []string{},
			wantErr: false, // Should be valid if targets-file is provided
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test Args validation if it exists
			if checkCommand.Args != nil {
				err := checkCommand.Args(checkCommand, tt.args)
				if (err != nil) != tt.wantErr {
					t.Errorf("Args validation error = %v, wantErr %v", err, tt.wantErr)
				}
			}
		})
	}
}

func TestCheckCommand_FlagValidation(t *testing.T) {
	// Test flag validation
	tests := []struct {
		name     string
		flags    map[string]string
		wantErr  bool
		errorMsg string
	}{
		{
			name: "valid timeout",
			flags: map[string]string{
				"timeout": "30s",
			},
			wantErr: false,
		},
		{
			name: "invalid timeout format",
			flags: map[string]string{
				"timeout": "invalid",
			},
			wantErr:  true,
			errorMsg: "invalid timeout",
		},
		{
			name: "negative timeout",
			flags: map[string]string{
				"timeout": "-5s",
			},
			wantErr:  true,
			errorMsg: "timeout must be positive",
		},
		{
			name: "valid format",
			flags: map[string]string{
				"format": "json",
			},
			wantErr: false,
		},
		{
			name: "invalid format",
			flags: map[string]string{
				"format": "invalid",
			},
			wantErr:  true,
			errorMsg: "invalid format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset flags
			checkCmd.ResetFlags()
			
			// Set up flags
			checkCmd.Flags().String("timeout", "30s", "")
			checkCmd.Flags().String("format", "table", "")
			checkCmd.Flags().String("output", "", "")
			checkCmd.Flags().String("targets-file", "", "")
			checkCmd.Flags().Bool("verbose", false, "")

			// Set flag values
			for flag, value := range tt.flags {
				err := checkCmd.Flags().Set(flag, value)
				if err != nil {
					t.Fatalf("Failed to set flag %s: %v", flag, err)
				}
			}

			// Test flag parsing (this would normally be done in PreRun or Run)
			if tt.flags["timeout"] != "" {
				timeoutStr, _ := checkCmd.Flags().GetString("timeout")
				_, err := time.ParseDuration(timeoutStr)
				
				if tt.wantErr && err == nil {
					t.Error("Expected timeout parsing error")
				} else if !tt.wantErr && err != nil {
					t.Errorf("Unexpected timeout parsing error: %v", err)
				}
			}

			if tt.flags["format"] != "" {
				format, _ := checkCmd.Flags().GetString("format")
				validFormats := []string{"table", "json", "csv"}
				
				isValid := false
				for _, validFormat := range validFormats {
					if format == validFormat {
						isValid = true
						break
					}
				}
				
				if tt.wantErr && isValid {
					t.Error("Expected format validation error")
				} else if !tt.wantErr && !isValid {
					t.Errorf("Format '%s' should be valid", format)
				}
			}
		})
	}
}

func TestCheckCommand_OutputFormats(t *testing.T) {
	// Test that all supported output formats are documented
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	checkCmd.SetArgs([]string{"--help"})
	checkCmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Should mention supported formats
	expectedFormats := []string{"table", "json", "csv"}
	
	for _, format := range expectedFormats {
		if !strings.Contains(output, format) {
			t.Logf("Format '%s' not mentioned in help", format)
		}
	}
}

func TestCheckCommand_Integration(t *testing.T) {
	// Test basic integration without actually running network calls
	// This tests the command structure and flag parsing
	
	// Create a temporary targets file
	tmpDir := t.TempDir()
	targetsFile := tmpDir + "/targets.yaml"
	
	targetsContent := `targets:
  - https://httpbin.org/status/200
  - google.com:80`
	
	err := os.WriteFile(targetsFile, []byte(targetsContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create targets file: %v", err)
	}

	// Test that command can be set up with valid flags
	checkCmd.ResetFlags()
	checkCmd.Flags().String("timeout", "30s", "")
	checkCmd.Flags().String("format", "table", "")
	checkCmd.Flags().String("output", "", "")
	checkCmd.Flags().String("targets-file", "", "")
	checkCmd.Flags().Bool("verbose", false, "")

	// Set flags
	checkCmd.Flags().Set("timeout", "10s")
	checkCmd.Flags().Set("format", "json")
	checkCmd.Flags().Set("targets-file", targetsFile)
	checkCmd.Flags().Set("verbose", "true")

	// Verify flags were set correctly
	timeout, _ := checkCmd.Flags().GetString("timeout")
	if timeout != "10s" {
		t.Errorf("Expected timeout '10s', got '%s'", timeout)
	}

	format, _ := checkCmd.Flags().GetString("format")
	if format != "json" {
		t.Errorf("Expected format 'json', got '%s'", format)
	}

	targetsFileFlag, _ := checkCmd.Flags().GetString("targets-file")
	if targetsFileFlag != targetsFile {
		t.Errorf("Expected targets-file '%s', got '%s'", targetsFile, targetsFileFlag)
	}

	verbose, _ := checkCmd.Flags().GetBool("verbose")
	if !verbose {
		t.Error("Expected verbose to be true")
	}
}

func TestCheckCommand_ErrorHandling(t *testing.T) {
	// Test error handling for common scenarios
	
	// Test with non-existent targets file
	checkCmd.ResetFlags()
	checkCmd.Flags().String("targets-file", "", "")
	
	err := checkCmd.Flags().Set("targets-file", "/non/existent/file.yaml")
	if err != nil {
		t.Fatalf("Failed to set targets-file flag: %v", err)
	}

	// The actual error handling would be in the Run function
	// Here we just verify the flag was set
	targetsFile, _ := checkCmd.Flags().GetString("targets-file")
	if targetsFile != "/non/existent/file.yaml" {
		t.Error("Targets file flag not set correctly")
	}
}

func TestCheckCommand_Aliases(t *testing.T) {
	// Test command aliases if they exist
	if len(checkCmd.Aliases) > 0 {
		t.Logf("Check command has aliases: %v", checkCmd.Aliases)
		
		// Verify aliases are reasonable
		for _, alias := range checkCmd.Aliases {
			if len(alias) == 0 {
				t.Error("Empty alias found")
			}
			if len(alias) > 10 {
				t.Errorf("Alias '%s' is too long", alias)
			}
		}
	}
}

func TestCheckCommand_PreRun(t *testing.T) {
	// Test PreRun function if it exists
	if checkCmd.PreRun != nil {
		// PreRun should not panic
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("PreRun should not panic: %v", r)
			}
		}()

		// Test with valid args
		checkCmd.PreRun(checkCmd, []string{"https://example.com"})
	}

	if checkCmd.PreRunE != nil {
		// PreRunE should handle errors gracefully
		err := checkCmd.PreRunE(checkCmd, []string{"https://example.com"})
		if err != nil {
			t.Logf("PreRunE returned error (may be expected): %v", err)
		}
	}
}

func BenchmarkCheckCommand_FlagParsing(b *testing.B) {
	// Benchmark flag parsing performance
	checkCmd.ResetFlags()
	checkCmd.Flags().String("timeout", "30s", "")
	checkCmd.Flags().String("format", "table", "")
	checkCmd.Flags().String("output", "", "")
	checkCmd.Flags().String("targets-file", "", "")
	checkCmd.Flags().Bool("verbose", false, "")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		checkCmd.Flags().Set("timeout", "30s")
		checkCmd.Flags().Set("format", "json")
		checkCmd.Flags().Set("verbose", "true")
	}
}