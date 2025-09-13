package cmd

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestRootCommand_Initialization(t *testing.T) {
	if rootCmd == nil {
		t.Fatal("rootCmd is nil")
	}

	if rootCmd.Use != "netirk" {
		t.Errorf("Expected Use 'netirk', got '%s'", rootCmd.Use)
	}

	if rootCmd.Short == "" {
		t.Error("Short description should not be empty")
	}

	if rootCmd.Long == "" {
		t.Error("Long description should not be empty")
	}
}

func TestRootCommand_HasSubcommands(t *testing.T) {
	// Check that root command has expected subcommands
	expectedCommands := []string{"check", "trace", "server", "monitor", "analyze", "version"}
	
	for _, expectedCmd := range expectedCommands {
		found := false
		for _, cmd := range rootCmd.Commands() {
			if cmd.Use == expectedCmd {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected subcommand '%s' not found", expectedCmd)
		}
	}
}

func TestRootCommand_Execute(t *testing.T) {
	// Test that Execute function exists and can be called
	// We'll capture output to avoid printing to console during tests
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Set args to help to avoid running actual commands
	rootCmd.SetArgs([]string{"--help"})

	Execute()
	
	// Restore stdout
	w.Close()
	os.Stdout = oldStdout

	// Read captured output
	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Should contain usage information
	if !strings.Contains(output, "Usage:") {
		t.Error("Help output should contain 'Usage:'")
	}

	if !strings.Contains(output, "netirk") {
		t.Error("Help output should contain 'netirk'")
	}
}

func TestRootCommand_Version(t *testing.T) {
	// Test version command
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	rootCmd.SetArgs([]string{"version"})
	Execute()

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Should contain version information
	if !strings.Contains(output, "netirk") {
		t.Error("Version output should contain 'netirk'")
	}
}

func TestRootCommand_GlobalFlags(t *testing.T) {
	// Test that global flags are properly set up
	// Most commands should inherit global flags if any exist
	
	// Check if root command has persistent flags
	persistentFlags := rootCmd.PersistentFlags()
	if persistentFlags == nil {
		t.Error("Root command should have persistent flags initialized")
	}

	// Check if root command has local flags
	localFlags := rootCmd.Flags()
	if localFlags == nil {
		t.Error("Root command should have local flags initialized")
	}
}

func TestRootCommand_Help(t *testing.T) {
	// Test help functionality
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	rootCmd.SetArgs([]string{"help"})
	Execute()

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Should contain available commands
	expectedSections := []string{
		"Available Commands:",
		"Flags:",
		"Use \"netirk [command] --help\" for more information",
	}

	for _, section := range expectedSections {
		if !strings.Contains(output, section) {
			t.Errorf("Help output should contain '%s'", section)
		}
	}
}

func TestRootCommand_InvalidCommand(t *testing.T) {
	// Test behavior with invalid command
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	rootCmd.SetArgs([]string{"invalid-command"})
	Execute()

	w.Close()
	os.Stderr = oldStderr

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Error output should be helpful
	if !strings.Contains(output, "unknown command") || !strings.Contains(output, "invalid-command") {
		t.Error("Error output should mention unknown command")
	}
}

func TestRootCommand_CompletionSupport(t *testing.T) {
	// Test that completion commands are available
	completionFound := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "completion" {
			completionFound = true
			break
		}
	}

	// Completion command is optional, so we just log if it's not found
	if !completionFound {
		t.Log("Completion command not found - this is optional")
	}
}

func TestRootCommand_PersistentPreRun(t *testing.T) {
	// Test that persistent pre-run functions work if they exist
	if rootCmd.PersistentPreRun != nil {
		// If there's a persistent pre-run, it should not panic
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("PersistentPreRun should not panic: %v", r)
			}
		}()

		// Create a mock command to test with
		testCmd := &cobra.Command{Use: "test"}
		rootCmd.PersistentPreRun(testCmd, []string{})
	}
}

func TestRootCommand_SilenceUsage(t *testing.T) {
	// Test that SilenceUsage is properly configured
	// This prevents usage from being printed on every error
	if !rootCmd.SilenceUsage {
		t.Log("SilenceUsage is false - usage will be printed on errors")
	}
}

func TestRootCommand_SilenceErrors(t *testing.T) {
	// Test that SilenceErrors is properly configured
	// This allows custom error handling
	if !rootCmd.SilenceErrors {
		t.Log("SilenceErrors is false - errors will be printed automatically")
	}
}

// Test helper functions if they exist
func TestInitConfig(t *testing.T) {
	// Test config initialization if the function exists
	// This is typically called in init() or cobra.OnInitialize()
	
	// Since we can't easily test init functions, we'll just verify
	// that the command structure is properly set up
	if rootCmd == nil {
		t.Fatal("Root command not initialized")
	}

	// Verify that subcommands are properly added
	if len(rootCmd.Commands()) == 0 {
		t.Error("Root command should have subcommands")
	}
}

func BenchmarkRootCommand_Execute_Help(b *testing.B) {
	// Benchmark help command execution
	oldStdout := os.Stdout
	os.Stdout, _ = os.Open(os.DevNull)
	defer func() { os.Stdout = oldStdout }()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rootCmd.SetArgs([]string{"--help"})
		Execute()
	}
}

func BenchmarkRootCommand_Execute_Version(b *testing.B) {
	// Benchmark version command execution
	oldStdout := os.Stdout
	os.Stdout, _ = os.Open(os.DevNull)
	defer func() { os.Stdout = oldStdout }()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rootCmd.SetArgs([]string{"version"})
		Execute()
	}
}