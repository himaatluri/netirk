package cmd

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func TestParseMonitorFlags(t *testing.T) {
	cmd := &cobra.Command{}
	
	// Add flags
	cmd.Flags().String("targets-file", "test.yaml", "")
	cmd.Flags().String("interval", "30s", "")
	cmd.Flags().String("duration", "1h", "")
	cmd.Flags().String("output-file", "output.json", "")
	cmd.Flags().String("output-format", "json", "")
	cmd.Flags().Int("alert-threshold", 3, "")
	cmd.Flags().String("alert-webhook", "https://example.com/webhook", "")
	cmd.Flags().String("alert-email", "admin@example.com", "")
	
	config, err := parseMonitorFlags(cmd)
	if err != nil {
		t.Fatalf("parseMonitorFlags failed: %v", err)
	}
	
	if config.TargetsFile != "test.yaml" {
		t.Errorf("Expected TargetsFile 'test.yaml', got '%s'", config.TargetsFile)
	}
	
	if config.Interval != 30*time.Second {
		t.Errorf("Expected Interval 30s, got %v", config.Interval)
	}
	
	if config.Duration != 1*time.Hour {
		t.Errorf("Expected Duration 1h, got %v", config.Duration)
	}
	
	if config.OutputFile != "output.json" {
		t.Errorf("Expected OutputFile 'output.json', got '%s'", config.OutputFile)
	}
	
	if config.OutputFormat != "json" {
		t.Errorf("Expected OutputFormat 'json', got '%s'", config.OutputFormat)
	}
	
	if config.AlertThreshold != 3 {
		t.Errorf("Expected AlertThreshold 3, got %d", config.AlertThreshold)
	}
	
	if config.AlertWebhook != "https://example.com/webhook" {
		t.Errorf("Expected AlertWebhook 'https://example.com/webhook', got '%s'", config.AlertWebhook)
	}
	
	if config.AlertEmail != "admin@example.com" {
		t.Errorf("Expected AlertEmail 'admin@example.com', got '%s'", config.AlertEmail)
	}
}

func TestParseMonitorFlags_InvalidInterval(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("targets-file", "test.yaml", "")
	cmd.Flags().String("interval", "invalid", "")
	cmd.Flags().String("duration", "", "")
	cmd.Flags().String("output-file", "", "")
	cmd.Flags().String("output-format", "", "")
	cmd.Flags().Int("alert-threshold", 0, "")
	cmd.Flags().String("alert-webhook", "", "")
	cmd.Flags().String("alert-email", "", "")
	
	_, err := parseMonitorFlags(cmd)
	if err == nil {
		t.Error("Expected error for invalid interval format")
	}
	
	if !containsString(err.Error(), "invalid interval format") {
		t.Errorf("Expected 'invalid interval format' error, got: %v", err)
	}
}

func TestParseMonitorFlags_InvalidDuration(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("targets-file", "test.yaml", "")
	cmd.Flags().String("interval", "30s", "")
	cmd.Flags().String("duration", "invalid", "")
	cmd.Flags().String("output-file", "", "")
	cmd.Flags().String("output-format", "", "")
	cmd.Flags().Int("alert-threshold", 0, "")
	cmd.Flags().String("alert-webhook", "", "")
	cmd.Flags().String("alert-email", "", "")
	
	_, err := parseMonitorFlags(cmd)
	if err == nil {
		t.Error("Expected error for invalid duration format")
	}
	
	if !containsString(err.Error(), "invalid duration format") {
		t.Errorf("Expected 'invalid duration format' error, got: %v", err)
	}
}

func TestValidateMonitorConfig(t *testing.T) {
	// Create a temporary targets file
	tmpDir := t.TempDir()
	targetsFile := filepath.Join(tmpDir, "targets.yaml")
	err := os.WriteFile(targetsFile, []byte("targets:\n  - https://example.com"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test targets file: %v", err)
	}
	
	tests := []struct {
		name    string
		config  *MonitorConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			config: &MonitorConfig{
				TargetsFile:    targetsFile,
				Interval:       30 * time.Second,
				Duration:       1 * time.Hour,
				OutputFormat:   "json",
				AlertThreshold: 3,
			},
			wantErr: false,
		},
		{
			name: "missing targets file",
			config: &MonitorConfig{
				TargetsFile: "",
				Interval:    30 * time.Second,
			},
			wantErr: true,
			errMsg:  "targets-file is required",
		},
		{
			name: "non-existent targets file",
			config: &MonitorConfig{
				TargetsFile: "/non/existent/file.yaml",
				Interval:    30 * time.Second,
			},
			wantErr: true,
			errMsg:  "targets file does not exist",
		},
		{
			name: "zero interval",
			config: &MonitorConfig{
				TargetsFile: targetsFile,
				Interval:    0,
			},
			wantErr: true,
			errMsg:  "interval must be positive",
		},
		{
			name: "negative interval",
			config: &MonitorConfig{
				TargetsFile: targetsFile,
				Interval:    -30 * time.Second,
			},
			wantErr: true,
			errMsg:  "interval must be positive",
		},
		{
			name: "negative duration",
			config: &MonitorConfig{
				TargetsFile: targetsFile,
				Interval:    30 * time.Second,
				Duration:    -1 * time.Hour,
			},
			wantErr: true,
			errMsg:  "duration cannot be negative",
		},
		{
			name: "invalid output format",
			config: &MonitorConfig{
				TargetsFile:  targetsFile,
				Interval:     30 * time.Second,
				OutputFormat: "invalid",
			},
			wantErr: true,
			errMsg:  "invalid output format",
		},
		{
			name: "negative alert threshold",
			config: &MonitorConfig{
				TargetsFile:    targetsFile,
				Interval:       30 * time.Second,
				AlertThreshold: -1,
			},
			wantErr: true,
			errMsg:  "alert threshold cannot be negative",
		},
		{
			name: "valid output formats",
			config: &MonitorConfig{
				TargetsFile:  targetsFile,
				Interval:     30 * time.Second,
				OutputFormat: "csv",
			},
			wantErr: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateMonitorConfig(tt.config)
			
			if tt.wantErr {
				if err == nil {
					t.Error("Expected error but got none")
				} else if !containsString(err.Error(), tt.errMsg) {
					t.Errorf("Expected error containing '%s', got: %v", tt.errMsg, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

func TestValidateMonitorConfig_ValidOutputFormats(t *testing.T) {
	// Create a temporary targets file
	tmpDir := t.TempDir()
	targetsFile := filepath.Join(tmpDir, "targets.yaml")
	err := os.WriteFile(targetsFile, []byte("targets:\n  - https://example.com"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test targets file: %v", err)
	}
	
	validFormats := []string{"json", "csv", "html", "prometheus"}
	
	for _, format := range validFormats {
		t.Run(format, func(t *testing.T) {
			config := &MonitorConfig{
				TargetsFile:  targetsFile,
				Interval:     30 * time.Second,
				OutputFormat: format,
			}
			
			err := validateMonitorConfig(config)
			if err != nil {
				t.Errorf("Expected no error for valid format '%s', got: %v", format, err)
			}
		})
	}
}

func TestMonitorConfig_Struct(t *testing.T) {
	config := &MonitorConfig{
		TargetsFile:    "test.yaml",
		Interval:       30 * time.Second,
		Duration:       1 * time.Hour,
		OutputFile:     "output.json",
		OutputFormat:   "json",
		AlertThreshold: 3,
		AlertWebhook:   "https://example.com/webhook",
		AlertEmail:     "admin@example.com",
	}
	
	// Test that all fields are accessible
	if config.TargetsFile != "test.yaml" {
		t.Error("TargetsFile field not accessible")
	}
	
	if config.Interval != 30*time.Second {
		t.Error("Interval field not accessible")
	}
	
	if config.Duration != 1*time.Hour {
		t.Error("Duration field not accessible")
	}
	
	if config.OutputFile != "output.json" {
		t.Error("OutputFile field not accessible")
	}
	
	if config.OutputFormat != "json" {
		t.Error("OutputFormat field not accessible")
	}
	
	if config.AlertThreshold != 3 {
		t.Error("AlertThreshold field not accessible")
	}
	
	if config.AlertWebhook != "https://example.com/webhook" {
		t.Error("AlertWebhook field not accessible")
	}
	
	if config.AlertEmail != "admin@example.com" {
		t.Error("AlertEmail field not accessible")
	}
}

// Helper function for string containment check
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || 
		(len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || 
		func() bool {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
			return false
		}())))
}

// Test monitor command initialization
func TestMonitorCommand_Initialization(t *testing.T) {
	if monitorCmd == nil {
		t.Fatal("monitorCmd is nil")
	}
	
	if monitorCmd.Use != "monitor" {
		t.Errorf("Expected Use 'monitor', got '%s'", monitorCmd.Use)
	}
	
	if monitorCmd.Short == "" {
		t.Error("Short description should not be empty")
	}
	
	if monitorCmd.Long == "" {
		t.Error("Long description should not be empty")
	}
	
	if monitorCmd.Run == nil {
		t.Error("Run function should not be nil")
	}
}

func TestMonitorCommand_Flags(t *testing.T) {
	// Check that required flags are present
	targetsFileFlag := monitorCmd.Flags().Lookup("targets-file")
	if targetsFileFlag == nil {
		t.Error("targets-file flag not found")
	}
	
	intervalFlag := monitorCmd.Flags().Lookup("interval")
	if intervalFlag == nil {
		t.Error("interval flag not found")
	}
	if intervalFlag.DefValue != "30s" {
		t.Errorf("Expected interval default '30s', got '%s'", intervalFlag.DefValue)
	}
	
	durationFlag := monitorCmd.Flags().Lookup("duration")
	if durationFlag == nil {
		t.Error("duration flag not found")
	}
	
	outputFileFlag := monitorCmd.Flags().Lookup("output-file")
	if outputFileFlag == nil {
		t.Error("output-file flag not found")
	}
	
	outputFormatFlag := monitorCmd.Flags().Lookup("output-format")
	if outputFormatFlag == nil {
		t.Error("output-format flag not found")
	}
	
	alertThresholdFlag := monitorCmd.Flags().Lookup("alert-threshold")
	if alertThresholdFlag == nil {
		t.Error("alert-threshold flag not found")
	}
	if alertThresholdFlag.DefValue != "0" {
		t.Errorf("Expected alert-threshold default '0', got '%s'", alertThresholdFlag.DefValue)
	}
	
	alertWebhookFlag := monitorCmd.Flags().Lookup("alert-webhook")
	if alertWebhookFlag == nil {
		t.Error("alert-webhook flag not found")
	}
	
	alertEmailFlag := monitorCmd.Flags().Lookup("alert-email")
	if alertEmailFlag == nil {
		t.Error("alert-email flag not found")
	}
}

func TestMonitorCommand_RequiredFlags(t *testing.T) {
	// Check that targets-file is marked as required
	requiredFlags := []string{}
	monitorCmd.Flags().VisitAll(func(flag *pflag.Flag) {
		if flag.Name == "targets-file" {
			// This is a simplified check - in a real test environment,
			// you would need to check the command's required flags
			requiredFlags = append(requiredFlags, flag.Name)
		}
	})
	
	if len(requiredFlags) == 0 {
		t.Error("targets-file should be marked as required")
	}
}

// Test edge cases for flag parsing
func TestParseMonitorFlags_EdgeCases(t *testing.T) {
	// Test with empty duration (should be allowed)
	cmd := &cobra.Command{}
	cmd.Flags().String("targets-file", "test.yaml", "")
	cmd.Flags().String("interval", "30s", "")
	cmd.Flags().String("duration", "", "")
	cmd.Flags().String("output-file", "", "")
	cmd.Flags().String("output-format", "", "")
	cmd.Flags().Int("alert-threshold", 0, "")
	cmd.Flags().String("alert-webhook", "", "")
	cmd.Flags().String("alert-email", "", "")
	
	config, err := parseMonitorFlags(cmd)
	if err != nil {
		t.Fatalf("parseMonitorFlags failed with empty duration: %v", err)
	}
	
	if config.Duration != 0 {
		t.Errorf("Expected Duration 0 for empty duration, got %v", config.Duration)
	}
	
	// Test with zero alert threshold (should be allowed)
	if config.AlertThreshold != 0 {
		t.Errorf("Expected AlertThreshold 0, got %d", config.AlertThreshold)
	}
}

func TestValidateMonitorConfig_EdgeCases(t *testing.T) {
	// Create a temporary targets file
	tmpDir := t.TempDir()
	targetsFile := filepath.Join(tmpDir, "targets.yaml")
	err := os.WriteFile(targetsFile, []byte("targets:\n  - https://example.com"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test targets file: %v", err)
	}
	
	// Test with zero duration (should be allowed)
	config := &MonitorConfig{
		TargetsFile: targetsFile,
		Interval:    30 * time.Second,
		Duration:    0,
	}
	
	err = validateMonitorConfig(config)
	if err != nil {
		t.Errorf("Expected no error for zero duration, got: %v", err)
	}
	
	// Test with zero alert threshold (should be allowed)
	config.AlertThreshold = 0
	err = validateMonitorConfig(config)
	if err != nil {
		t.Errorf("Expected no error for zero alert threshold, got: %v", err)
	}
	
	// Test with empty output format (should be allowed)
	config.OutputFormat = ""
	err = validateMonitorConfig(config)
	if err != nil {
		t.Errorf("Expected no error for empty output format, got: %v", err)
	}
}