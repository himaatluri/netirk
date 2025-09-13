package helpers

import (
	"os"
	"testing"
	"time"
)

func TestValidateURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"valid https", "https://example.com", false},
		{"valid http", "http://example.com", false},
		{"valid tcp", "tcp://localhost:8080", false},
		{"empty url", "", true},
		{"invalid scheme", "ftp://example.com", true},
		{"no host", "https://", true},
		{"tcp without port", "tcp://localhost", true},
		{"tcp with invalid port", "tcp://localhost:abc", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateURL() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateHeaders(t *testing.T) {
	tests := []struct {
		name    string
		headers map[string]string
		wantErr bool
	}{
		{"nil headers", nil, false},
		{"empty headers", map[string]string{}, false},
		{"valid headers", map[string]string{"Authorization": "Bearer token", "User-Agent": "test"}, false},
		{"empty header name", map[string]string{"": "value"}, true},
		{"invalid header name", map[string]string{"Invalid Header": "value"}, true},
		{"control characters in value", map[string]string{"Test": "value\r\n"}, true},
		{"forbidden header", map[string]string{"Host": "example.com"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateHeaders(tt.headers)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateHeaders() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateAndParseTarget(t *testing.T) {
	tests := []struct {
		name    string
		config  TargetConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: TargetConfig{
				URL:            "https://example.com",
				Timeout:        "30s",
				Interval:       "60s",
				ExpectedStatus: 200,
				SSLCheck:       true,
			},
			wantErr: false,
		},
		{
			name: "missing url",
			config: TargetConfig{
				Timeout: "30s",
			},
			wantErr: true,
		},
		{
			name: "invalid timeout",
			config: TargetConfig{
				URL:     "https://example.com",
				Timeout: "invalid",
			},
			wantErr: true,
		},
		{
			name: "negative timeout",
			config: TargetConfig{
				URL:     "https://example.com",
				Timeout: "-30s",
			},
			wantErr: true,
		},
		{
			name: "timeout too large",
			config: TargetConfig{
				URL:     "https://example.com",
				Timeout: "10m",
			},
			wantErr: true,
		},
		{
			name: "invalid status code",
			config: TargetConfig{
				URL:            "https://example.com",
				ExpectedStatus: 999,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := validateAndParseTarget(tt.config, 0)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateAndParseTarget() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGenerateConfigurationExample(t *testing.T) {
	// Create a temporary file
	tmpFile := "test-config.yaml"
	defer os.Remove(tmpFile)

	err := GenerateConfigurationExample(tmpFile)
	if err != nil {
		t.Errorf("GenerateConfigurationExample() error = %v", err)
	}

	// Check if file was created
	if _, err := os.Stat(tmpFile); os.IsNotExist(err) {
		t.Error("Configuration example file was not created")
	}

	// Try to parse the generated file
	_, err = ParseEnhancedTargetFile(tmpFile)
	if err != nil {
		t.Errorf("Generated configuration file is not valid: %v", err)
	}
}

func TestValidationError(t *testing.T) {
	err := NewValidationError("test message", "test_code", "test suggestion")
	
	if err.Error() != "test message" {
		t.Errorf("ValidationError.Error() = %v, want %v", err.Error(), "test message")
	}

	expectedMsg := "test message\n  Suggestion: test suggestion"
	if err.GetUserFriendlyMessage() != expectedMsg {
		t.Errorf("ValidationError.GetUserFriendlyMessage() = %v, want %v", err.GetUserFriendlyMessage(), expectedMsg)
	}
}

func TestParseEnhancedTargetFileWithValidation(t *testing.T) {
	// Create a test configuration file
	testConfig := `targets:
  - url: "https://example.com"
    timeout: "30s"
    interval: "60s"
    expected_status: 200
    ssl_check: true
  - url: "tcp://localhost:8080"
    timeout: "5s"
`

	tmpFile := "test-enhanced-config.yaml"
	defer os.Remove(tmpFile)

	err := os.WriteFile(tmpFile, []byte(testConfig), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	targets, err := ParseEnhancedTargetFile(tmpFile)
	if err != nil {
		t.Errorf("ParseEnhancedTargetFile() error = %v", err)
	}

	if len(targets.Targets) != 2 {
		t.Errorf("Expected 2 targets, got %d", len(targets.Targets))
	}

	// Check first target
	target1 := targets.Targets[0]
	if target1.URL != "https://example.com" {
		t.Errorf("Expected URL 'https://example.com', got '%s'", target1.URL)
	}
	if target1.Timeout != 30*time.Second {
		t.Errorf("Expected timeout 30s, got %v", target1.Timeout)
	}
	if target1.Interval != 60*time.Second {
		t.Errorf("Expected interval 60s, got %v", target1.Interval)
	}
	if target1.ExpectedStatus != 200 {
		t.Errorf("Expected status 200, got %d", target1.ExpectedStatus)
	}
	if !target1.SSLCheck {
		t.Error("Expected SSL check to be true")
	}

	// Check second target
	target2 := targets.Targets[1]
	if target2.URL != "tcp://localhost:8080" {
		t.Errorf("Expected URL 'tcp://localhost:8080', got '%s'", target2.URL)
	}
	if target2.Timeout != 5*time.Second {
		t.Errorf("Expected timeout 5s, got %v", target2.Timeout)
	}
}