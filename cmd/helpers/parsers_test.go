package helpers

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseEnhancedTargetFile_LegacyFormat(t *testing.T) {
	// Create temporary file with legacy format
	content := `targets:
  - https://google.com
  - https://bing.com`

	tmpFile := createTempFile(t, content)
	defer os.Remove(tmpFile)

	result, err := ParseEnhancedTargetFile(tmpFile)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(result.Targets) != 2 {
		t.Fatalf("Expected 2 targets, got %d", len(result.Targets))
	}

	// Check first target
	target1 := result.Targets[0]
	if target1.URL != "https://google.com" {
		t.Errorf("Expected URL 'https://google.com', got '%s'", target1.URL)
	}
	if target1.Timeout != 30*time.Second {
		t.Errorf("Expected default timeout 30s, got %v", target1.Timeout)
	}
	if target1.Interval != 60*time.Second {
		t.Errorf("Expected default interval 60s, got %v", target1.Interval)
	}
	if target1.ExpectedStatus != 200 {
		t.Errorf("Expected default status 200, got %d", target1.ExpectedStatus)
	}
	if target1.SSLCheck != false {
		t.Errorf("Expected default SSL check false, got %v", target1.SSLCheck)
	}
}

func TestParseEnhancedTargetFile_EnhancedFormat(t *testing.T) {
	content := `targets:
  - url: "https://api.example.com"
    timeout: "30s"
    interval: "60s"
    expected_status: 200
    headers:
      Authorization: "Bearer token"
      User-Agent: "Netirk Monitor"
    ssl_check: true
  - url: "https://web.example.com"
    timeout: "10s"
    expected_status: 201
    ssl_check: false`

	tmpFile := createTempFile(t, content)
	defer os.Remove(tmpFile)

	result, err := ParseEnhancedTargetFile(tmpFile)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(result.Targets) != 2 {
		t.Fatalf("Expected 2 targets, got %d", len(result.Targets))
	}

	// Check first target
	target1 := result.Targets[0]
	if target1.URL != "https://api.example.com" {
		t.Errorf("Expected URL 'https://api.example.com', got '%s'", target1.URL)
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
		t.Errorf("Expected SSL check true, got %v", target1.SSLCheck)
	}
	if target1.Headers["Authorization"] != "Bearer token" {
		t.Errorf("Expected Authorization header 'Bearer token', got '%s'", target1.Headers["Authorization"])
	}
	if target1.Headers["User-Agent"] != "Netirk Monitor" {
		t.Errorf("Expected User-Agent header 'Netirk Monitor', got '%s'", target1.Headers["User-Agent"])
	}

	// Check second target (with defaults)
	target2 := result.Targets[1]
	if target2.URL != "https://web.example.com" {
		t.Errorf("Expected URL 'https://web.example.com', got '%s'", target2.URL)
	}
	if target2.Timeout != 10*time.Second {
		t.Errorf("Expected timeout 10s, got %v", target2.Timeout)
	}
	if target2.Interval != 60*time.Second {
		t.Errorf("Expected default interval 60s, got %v", target2.Interval)
	}
	if target2.ExpectedStatus != 201 {
		t.Errorf("Expected status 201, got %d", target2.ExpectedStatus)
	}
	if target2.SSLCheck != false {
		t.Errorf("Expected SSL check false, got %v", target2.SSLCheck)
	}
}

func TestParseEnhancedTargetFile_MixedFormat(t *testing.T) {
	content := `targets:
  - https://simple.com
  - url: "https://complex.com"
    timeout: "15s"
    expected_status: 404
    headers:
      Custom-Header: "value"`

	tmpFile := createTempFile(t, content)
	defer os.Remove(tmpFile)

	result, err := ParseEnhancedTargetFile(tmpFile)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(result.Targets) != 2 {
		t.Fatalf("Expected 2 targets, got %d", len(result.Targets))
	}

	// Check simple target
	target1 := result.Targets[0]
	if target1.URL != "https://simple.com" {
		t.Errorf("Expected URL 'https://simple.com', got '%s'", target1.URL)
	}
	if target1.ExpectedStatus != 200 {
		t.Errorf("Expected default status 200, got %d", target1.ExpectedStatus)
	}

	// Check complex target
	target2 := result.Targets[1]
	if target2.URL != "https://complex.com" {
		t.Errorf("Expected URL 'https://complex.com', got '%s'", target2.URL)
	}
	if target2.Timeout != 15*time.Second {
		t.Errorf("Expected timeout 15s, got %v", target2.Timeout)
	}
	if target2.ExpectedStatus != 404 {
		t.Errorf("Expected status 404, got %d", target2.ExpectedStatus)
	}
	if target2.Headers["Custom-Header"] != "value" {
		t.Errorf("Expected Custom-Header 'value', got '%s'", target2.Headers["Custom-Header"])
	}
}

func TestParseEnhancedTargetFile_ValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr string
	}{
		{
			name: "missing URL",
			content: `targets:
  - timeout: "30s"`,
			wantErr: "URL is required",
		},
		{
			name: "invalid timeout format",
			content: `targets:
  - url: "https://example.com"
    timeout: "invalid"`,
			wantErr: "invalid timeout format",
		},
		{
			name: "negative timeout",
			content: `targets:
  - url: "https://example.com"
    timeout: "-5s"`,
			wantErr: "timeout must be positive",
		},
		{
			name: "invalid interval format",
			content: `targets:
  - url: "https://example.com"
    interval: "bad-interval"`,
			wantErr: "invalid interval format",
		},
		{
			name: "negative interval",
			content: `targets:
  - url: "https://example.com"
    interval: "-10s"`,
			wantErr: "interval must be positive",
		},
		{
			name: "invalid status code - too low",
			content: `targets:
  - url: "https://example.com"
    expected_status: 99`,
			wantErr: "expected_status must be between 100-599",
		},
		{
			name: "invalid status code - too high",
			content: `targets:
  - url: "https://example.com"
    expected_status: 600`,
			wantErr: "expected_status must be between 100-599",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile := createTempFile(t, tt.content)
			defer os.Remove(tmpFile)

			_, err := ParseEnhancedTargetFile(tmpFile)
			if err == nil {
				t.Fatalf("Expected error containing '%s', got no error", tt.wantErr)
			}
			if !containsString(err.Error(), tt.wantErr) {
				t.Errorf("Expected error containing '%s', got '%s'", tt.wantErr, err.Error())
			}
		})
	}
}

func TestParseEnhancedTargetFile_FileErrors(t *testing.T) {
	// Test non-existent file
	_, err := ParseEnhancedTargetFile("non-existent-file.yaml")
	if err == nil {
		t.Error("Expected error for non-existent file, got nil")
	}
	if !containsString(err.Error(), "failed to read target file") {
		t.Errorf("Expected 'failed to read target file' error, got: %v", err)
	}

	// Test invalid YAML
	invalidYAML := `targets:
  - url: "https://example.com"
    invalid: yaml: content`
	tmpFile := createTempFile(t, invalidYAML)
	defer os.Remove(tmpFile)

	_, err = ParseEnhancedTargetFile(tmpFile)
	if err == nil {
		t.Error("Expected error for invalid YAML, got nil")
	}
	if !containsString(err.Error(), "failed to parse YAML") {
		t.Errorf("Expected 'failed to parse YAML' error, got: %v", err)
	}
}

func TestParseEnhancedTargetFile_InvalidTargetFormat(t *testing.T) {
	content := `targets:
  - 123
  - url: "https://example.com"`

	tmpFile := createTempFile(t, content)
	defer os.Remove(tmpFile)

	_, err := ParseEnhancedTargetFile(tmpFile)
	if err == nil {
		t.Error("Expected error for invalid target format, got nil")
	}
	if !containsString(err.Error(), "invalid target format") {
		t.Errorf("Expected 'invalid target format' error, got: %v", err)
	}
}

func TestParseTargetFile_LegacyCompatibility(t *testing.T) {
	// Test that the legacy function still works
	content := `targets:
  - https://google.com
  - https://bing.com`

	tmpFile := createTempFile(t, content)
	defer os.Remove(tmpFile)

	result := ParseTargetFile(tmpFile)
	if len(result.Targets) != 2 {
		t.Fatalf("Expected 2 targets, got %d", len(result.Targets))
	}

	// The legacy function returns []interface{}, so we need to type assert
	if result.Targets[0].(string) != "https://google.com" {
		t.Errorf("Expected 'https://google.com', got %v", result.Targets[0])
	}
	if result.Targets[1].(string) != "https://bing.com" {
		t.Errorf("Expected 'https://bing.com', got %v", result.Targets[1])
	}
}

func TestValidateAndParseTarget_EdgeCases(t *testing.T) {
	tests := []struct {
		name   string
		config TargetConfig
		want   ParsedTargetConfig
	}{
		{
			name: "minimal config",
			config: TargetConfig{
				URL: "https://example.com",
			},
			want: ParsedTargetConfig{
				URL:            "https://example.com",
				Timeout:        30 * time.Second,
				Interval:       60 * time.Second,
				ExpectedStatus: 200,
				Headers:        make(map[string]string),
				SSLCheck:       false,
			},
		},
		{
			name: "all fields specified",
			config: TargetConfig{
				URL:            "https://api.example.com",
				Timeout:        "45s",
				Interval:       "2m",
				ExpectedStatus: 201,
				Headers:        map[string]string{"Auth": "token"},
				SSLCheck:       true,
			},
			want: ParsedTargetConfig{
				URL:            "https://api.example.com",
				Timeout:        45 * time.Second,
				Interval:       2 * time.Minute,
				ExpectedStatus: 201,
				Headers:        map[string]string{"Auth": "token"},
				SSLCheck:       true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := validateAndParseTarget(tt.config, 0)
			if err != nil {
				t.Fatalf("Expected no error, got: %v", err)
			}

			if got.URL != tt.want.URL {
				t.Errorf("URL: got %s, want %s", got.URL, tt.want.URL)
			}
			if got.Timeout != tt.want.Timeout {
				t.Errorf("Timeout: got %v, want %v", got.Timeout, tt.want.Timeout)
			}
			if got.Interval != tt.want.Interval {
				t.Errorf("Interval: got %v, want %v", got.Interval, tt.want.Interval)
			}
			if got.ExpectedStatus != tt.want.ExpectedStatus {
				t.Errorf("ExpectedStatus: got %d, want %d", got.ExpectedStatus, tt.want.ExpectedStatus)
			}
			if got.SSLCheck != tt.want.SSLCheck {
				t.Errorf("SSLCheck: got %v, want %v", got.SSLCheck, tt.want.SSLCheck)
			}
		})
	}
}

// Helper functions

func createTempFile(t *testing.T, content string) string {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test-targets.yaml")
	
	err := os.WriteFile(tmpFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	
	return tmpFile
}

func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}