package helpers

import (
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	yaml "gopkg.in/yaml.v3"
)

// TargetConfig represents enhanced configuration for individual targets
type TargetConfig struct {
	URL            string            `yaml:"url"`
	Timeout        string            `yaml:"timeout,omitempty"`
	Interval       string            `yaml:"interval,omitempty"`
	ExpectedStatus int               `yaml:"expected_status,omitempty"`
	Headers        map[string]string `yaml:"headers,omitempty"`
	SSLCheck       bool              `yaml:"ssl_check,omitempty"`
}

// ParsedTargetConfig represents a target config with parsed duration values
type ParsedTargetConfig struct {
	URL            string
	Timeout        time.Duration
	Interval       time.Duration
	ExpectedStatus int
	Headers        map[string]string
	SSLCheck       bool
}

// Targets represents the root configuration structure
type Targets struct {
	Targets []interface{} `yaml:"targets,omitempty"`
}

// EnhancedTargets represents the enhanced configuration structure
type EnhancedTargets struct {
	Targets []ParsedTargetConfig
}

// ParseTargetFile parses the target configuration file and returns legacy format
func ParseTargetFile(path string) Targets {
	content, err := os.ReadFile(path)
	if err != nil {
		fmt.Println(err)
		return Targets{}
	}

	hostTargets := Targets{}
	yaml.Unmarshal(content, &hostTargets)

	return hostTargets
}

// ParseEnhancedTargetFile parses the target configuration file with enhanced format support
func ParseEnhancedTargetFile(path string) (EnhancedTargets, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return EnhancedTargets{}, fmt.Errorf("failed to read target file: %w", err)
	}

	var rawTargets Targets
	if err := yaml.Unmarshal(content, &rawTargets); err != nil {
		return EnhancedTargets{}, fmt.Errorf("failed to parse YAML: %w", err)
	}

	var parsedTargets []ParsedTargetConfig
	
	for i, target := range rawTargets.Targets {
		switch v := target.(type) {
		case string:
			// Legacy format - simple string URL
			parsedTarget := ParsedTargetConfig{
				URL:            v,
				Timeout:        30 * time.Second, // Default timeout
				Interval:       60 * time.Second, // Default interval
				ExpectedStatus: 200,              // Default expected status
				Headers:        make(map[string]string),
				SSLCheck:       false,
			}
			parsedTargets = append(parsedTargets, parsedTarget)
			
		case map[string]interface{}:
			// Enhanced format - structured configuration
			targetConfig := TargetConfig{}
			
			// Convert map to bytes and unmarshal to struct for proper type handling
			targetBytes, err := yaml.Marshal(v)
			if err != nil {
				return EnhancedTargets{}, fmt.Errorf("failed to marshal target %d: %w", i, err)
			}
			
			if err := yaml.Unmarshal(targetBytes, &targetConfig); err != nil {
				return EnhancedTargets{}, fmt.Errorf("failed to unmarshal target %d: %w", i, err)
			}
			
			parsedTarget, err := validateAndParseTarget(targetConfig, i)
			if err != nil {
				return EnhancedTargets{}, err
			}
			
			parsedTargets = append(parsedTargets, parsedTarget)
			
		default:
			return EnhancedTargets{}, fmt.Errorf("invalid target format at index %d: expected string or object", i)
		}
	}

	return EnhancedTargets{Targets: parsedTargets}, nil
}

// validateAndParseTarget validates and parses a single target configuration
func validateAndParseTarget(config TargetConfig, index int) (ParsedTargetConfig, error) {
	parsed := ParsedTargetConfig{
		URL:            config.URL,
		ExpectedStatus: config.ExpectedStatus,
		Headers:        config.Headers,
		SSLCheck:       config.SSLCheck,
	}

	// Validate required URL field
	if config.URL == "" {
		return ParsedTargetConfig{}, NewValidationError(fmt.Sprintf("target %d: URL is required", index), "missing_url", 
			"Each target must have a 'url' field. Example: url: \"https://example.com\"")
	}

	// Validate URL format
	if err := validateURL(config.URL); err != nil {
		return ParsedTargetConfig{}, NewValidationError(fmt.Sprintf("target %d: invalid URL '%s'", index, config.URL), "invalid_url", 
			fmt.Sprintf("URL validation failed: %v. Supported formats: https://example.com, http://example.com, tcp://host:port", err))
	}

	// Parse and validate timeout
	if config.Timeout != "" {
		timeout, err := time.ParseDuration(config.Timeout)
		if err != nil {
			return ParsedTargetConfig{}, NewValidationError(fmt.Sprintf("target %d: invalid timeout format '%s'", index, config.Timeout), "invalid_timeout", 
				"Timeout must be a valid duration string. Examples: \"30s\", \"1m\", \"500ms\"")
		}
		if timeout <= 0 {
			return ParsedTargetConfig{}, NewValidationError(fmt.Sprintf("target %d: timeout must be positive, got %v", index, timeout), "negative_timeout", 
				"Timeout must be a positive duration. Examples: \"1s\", \"30s\", \"1m\"")
		}
		if timeout > 5*time.Minute {
			return ParsedTargetConfig{}, NewValidationError(fmt.Sprintf("target %d: timeout %v exceeds maximum allowed (5m)", index, timeout), "timeout_too_large", 
				"Timeout cannot exceed 5 minutes. Consider using a shorter timeout for better monitoring responsiveness.")
		}
		parsed.Timeout = timeout
	} else {
		parsed.Timeout = 30 * time.Second // Default timeout
	}

	// Parse and validate interval
	if config.Interval != "" {
		interval, err := time.ParseDuration(config.Interval)
		if err != nil {
			return ParsedTargetConfig{}, NewValidationError(fmt.Sprintf("target %d: invalid interval format '%s'", index, config.Interval), "invalid_interval", 
				"Interval must be a valid duration string. Examples: \"30s\", \"1m\", \"5m\"")
		}
		if interval <= 0 {
			return ParsedTargetConfig{}, NewValidationError(fmt.Sprintf("target %d: interval must be positive, got %v", index, interval), "negative_interval", 
				"Interval must be a positive duration. Examples: \"30s\", \"1m\", \"5m\"")
		}
		if interval < 5*time.Second {
			return ParsedTargetConfig{}, NewValidationError(fmt.Sprintf("target %d: interval %v is too short (minimum 5s)", index, interval), "interval_too_short", 
				"Minimum monitoring interval is 5 seconds to avoid overwhelming target systems.")
		}
		parsed.Interval = interval
	} else {
		parsed.Interval = 60 * time.Second // Default interval
	}

	// Validate expected status code
	if config.ExpectedStatus != 0 {
		if config.ExpectedStatus < 100 || config.ExpectedStatus > 599 {
			return ParsedTargetConfig{}, NewValidationError(fmt.Sprintf("target %d: expected_status must be between 100-599, got %d", index, config.ExpectedStatus), "invalid_status_code", 
				"HTTP status codes must be between 100-599. Common values: 200 (OK), 201 (Created), 204 (No Content)")
		}
		parsed.ExpectedStatus = config.ExpectedStatus
	} else {
		parsed.ExpectedStatus = 200 // Default expected status
	}

	// Validate headers
	if err := validateHeaders(config.Headers); err != nil {
		return ParsedTargetConfig{}, NewValidationError(fmt.Sprintf("target %d: invalid headers", index), "invalid_headers", err.Error())
	}

	// Initialize headers map if nil
	if parsed.Headers == nil {
		parsed.Headers = make(map[string]string)
	}

	return parsed, nil
}

// ValidationError represents a configuration validation error with helpful context
type ValidationError struct {
	Message     string `json:"message"`
	ErrorCode   string `json:"error_code"`
	Suggestion  string `json:"suggestion"`
	FieldPath   string `json:"field_path,omitempty"`
}

func (e ValidationError) Error() string {
	return e.Message
}

// GetUserFriendlyMessage returns a user-friendly error message with suggestions
func (e ValidationError) GetUserFriendlyMessage() string {
	msg := e.Message
	if e.Suggestion != "" {
		msg += "\n  Suggestion: " + e.Suggestion
	}
	return msg
}

// NewValidationError creates a new validation error with context
func NewValidationError(message, errorCode, suggestion string) ValidationError {
	return ValidationError{
		Message:    message,
		ErrorCode:  errorCode,
		Suggestion: suggestion,
	}
}

// validateURL validates URL format and supported schemes
func validateURL(urlStr string) error {
	if urlStr == "" {
		return fmt.Errorf("URL cannot be empty")
	}

	// Handle TCP URLs specially
	if strings.HasPrefix(urlStr, "tcp://") {
		return validateTCPURL(urlStr)
	}

	// Parse HTTP/HTTPS URLs
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return fmt.Errorf("invalid URL format: %w", err)
	}

	// Validate scheme
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("unsupported scheme '%s', use http, https, or tcp", parsedURL.Scheme)
	}

	// Validate host
	if parsedURL.Host == "" {
		return fmt.Errorf("URL must include a host")
	}

	return nil
}

// validateTCPURL validates TCP URL format (tcp://host:port)
func validateTCPURL(urlStr string) error {
	// Remove tcp:// prefix
	hostPort := strings.TrimPrefix(urlStr, "tcp://")
	
	// Validate host:port format
	parts := strings.Split(hostPort, ":")
	if len(parts) != 2 {
		return fmt.Errorf("TCP URL must be in format tcp://host:port")
	}

	host, port := parts[0], parts[1]
	
	if host == "" {
		return fmt.Errorf("TCP URL must include a host")
	}
	
	if port == "" {
		return fmt.Errorf("TCP URL must include a port")
	}

	// Validate port is numeric
	if matched, _ := regexp.MatchString(`^\d+$`, port); !matched {
		return fmt.Errorf("TCP port must be numeric")
	}

	return nil
}

// validateHeaders validates HTTP headers format and content
func validateHeaders(headers map[string]string) error {
	if headers == nil {
		return nil
	}

	for name, value := range headers {
		// Validate header name
		if name == "" {
			return fmt.Errorf("header name cannot be empty")
		}

		// Check for invalid characters in header name
		if matched, _ := regexp.MatchString(`[^\w\-]`, name); matched {
			return fmt.Errorf("header name '%s' contains invalid characters. Use only letters, numbers, and hyphens", name)
		}

		// Validate header value (basic check for control characters)
		if strings.ContainsAny(value, "\r\n\t") {
			return fmt.Errorf("header value for '%s' contains invalid control characters", name)
		}

		// Warn about potentially problematic headers
		lowerName := strings.ToLower(name)
		if lowerName == "host" || lowerName == "content-length" || lowerName == "connection" {
			return fmt.Errorf("header '%s' is automatically managed and should not be set manually", name)
		}
	}

	return nil
}

// ValidateConfigurationFile performs comprehensive validation of a configuration file
func ValidateConfigurationFile(filename string) error {
	// Check if file exists
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return NewValidationError(fmt.Sprintf("configuration file not found: %s", filename), "file_not_found", 
			"Ensure the file path is correct and the file exists. Use 'netirk help monitor' for configuration examples.")
	}

	// Try to parse the file
	_, err := ParseEnhancedTargetFile(filename)
	if err != nil {
		// If it's already a ValidationError, return as-is
		if validationErr, ok := err.(ValidationError); ok {
			return validationErr
		}
		
		// Wrap other errors with helpful context
		return NewValidationError(fmt.Sprintf("configuration validation failed: %v", err), "validation_failed", 
			"Check the YAML syntax and ensure all required fields are present. Use 'netirk help monitor' for examples.")
	}

	return nil
}

// GenerateConfigurationExample creates an example configuration file
func GenerateConfigurationExample(filename string) error {
	exampleConfig := `# Netirk Enhanced Monitoring Configuration
# This file defines targets for network monitoring with advanced options

targets:
  # Simple HTTP target with default settings
  - url: "https://httpbin.org/status/200"
    
  # HTTP target with custom settings
  - url: "https://api.example.com/health"
    timeout: "10s"           # Request timeout (default: 30s)
    interval: "30s"          # Monitoring interval (default: 60s)
    expected_status: 200     # Expected HTTP status code (default: 200)
    ssl_check: true          # Enable SSL certificate monitoring (default: false)
    headers:                 # Custom HTTP headers
      Authorization: "Bearer your-token-here"
      User-Agent: "Netirk Monitor v1.0"
      Accept: "application/json"
  
  # TCP connection monitoring
  - url: "tcp://8.8.8.8:53"
    timeout: "5s"
    interval: "60s"
  
  # HTTPS with SSL monitoring
  - url: "https://expired.badssl.com"
    timeout: "15s"
    interval: "120s"
    ssl_check: true
    expected_status: 200

# Alert configuration (optional)
alert_config:
  enabled: true
  failure_threshold: 3       # Trigger alert after N consecutive failures
  ssl_warning_days: 30       # Warn when SSL cert expires within N days
  ssl_critical_days: 7       # Critical alert when SSL cert expires within N days
  
  # Webhook notifications
  webhook_url: "https://hooks.slack.com/services/YOUR/SLACK/WEBHOOK"
  
  # Email notifications
  email_recipients:
    - "admin@example.com"
    - "devops@example.com"
  
  email_config:
    smtp_host: "smtp.gmail.com"
    smtp_port: 587
    username: "alerts@example.com"
    password: "your-app-password"
    from_address: "alerts@example.com"
    from_name: "Netirk Network Monitor"
    use_starttls: true
    insecure_tls: false

# Export configuration (optional)
export_config:
  auto_export: true
  formats: ["json", "csv"]   # Available: json, csv, html, prometheus
  output_directory: "./monitoring-reports"
`

	return os.WriteFile(filename, []byte(exampleConfig), 0644)
}
