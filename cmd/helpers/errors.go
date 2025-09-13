package helpers

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"
)

// ErrorType represents different categories of errors
type ErrorType string

const (
	ErrorTypeNetwork    ErrorType = "network"
	ErrorTypeStorage    ErrorType = "storage"
	ErrorTypeAlert      ErrorType = "alert"
	ErrorTypeConfig     ErrorType = "config"
	ErrorTypeSSL        ErrorType = "ssl"
	ErrorTypeTimeout    ErrorType = "timeout"
	ErrorTypeValidation ErrorType = "validation"
)

// ErrorSeverity represents the severity level of an error
type ErrorSeverity string

const (
	ErrorSeverityLow      ErrorSeverity = "low"
	ErrorSeverityMedium   ErrorSeverity = "medium"
	ErrorSeverityHigh     ErrorSeverity = "high"
	ErrorSeverityCritical ErrorSeverity = "critical"
)

// NetirkError represents a structured error with context and recovery information
type NetirkError struct {
	Type        ErrorType     `json:"type"`
	Severity    ErrorSeverity `json:"severity"`
	Message     string        `json:"message"`
	UserMessage string        `json:"user_message"`
	Target      string        `json:"target,omitempty"`
	Timestamp   time.Time     `json:"timestamp"`
	Cause       error         `json:"cause,omitempty"`
	Context     string        `json:"context,omitempty"`
	Recoverable bool          `json:"recoverable"`
	RetryAfter  time.Duration `json:"retry_after,omitempty"`
}

// Error implements the error interface
func (e *NetirkError) Error() string {
	if e.Target != "" {
		return fmt.Sprintf("[%s:%s] %s (target: %s)", e.Type, e.Severity, e.Message, e.Target)
	}
	return fmt.Sprintf("[%s:%s] %s", e.Type, e.Severity, e.Message)
}

// Unwrap returns the underlying cause error
func (e *NetirkError) Unwrap() error {
	return e.Cause
}

// GetUserFriendlyMessage returns a user-friendly error message
func (e *NetirkError) GetUserFriendlyMessage() string {
	if e.UserMessage != "" {
		return e.UserMessage
	}
	return e.Message
}

// IsRecoverable returns whether the error is recoverable
func (e *NetirkError) IsRecoverable() bool {
	return e.Recoverable
}

// GetRetryDelay returns the recommended retry delay
func (e *NetirkError) GetRetryDelay() time.Duration {
	if e.RetryAfter > 0 {
		return e.RetryAfter
	}
	
	// Default retry delays based on error type and severity
	switch e.Type {
	case ErrorTypeNetwork:
		switch e.Severity {
		case ErrorSeverityLow:
			return 5 * time.Second
		case ErrorSeverityMedium:
			return 15 * time.Second
		case ErrorSeverityHigh:
			return 30 * time.Second
		case ErrorSeverityCritical:
			return 60 * time.Second
		}
	case ErrorTypeStorage:
		return 10 * time.Second
	case ErrorTypeAlert:
		return 30 * time.Second
	default:
		return 15 * time.Second
	}
	
	return 15 * time.Second
}

// ErrorHandler provides centralized error handling and recovery mechanisms
type ErrorHandler struct {
	logger        *log.Logger
	retryAttempts map[string]int
	lastErrors    map[string]*NetirkError
	maxRetries    int
}

// NewErrorHandler creates a new error handler
func NewErrorHandler(logger *log.Logger) *ErrorHandler {
	if logger == nil {
		logger = log.Default()
	}
	
	return &ErrorHandler{
		logger:        logger,
		retryAttempts: make(map[string]int),
		lastErrors:    make(map[string]*NetirkError),
		maxRetries:    3,
	}
}

// SetMaxRetries sets the maximum number of retry attempts
func (eh *ErrorHandler) SetMaxRetries(maxRetries int) {
	eh.maxRetries = maxRetries
}

// HandleError processes an error and determines recovery actions
func (eh *ErrorHandler) HandleError(err error, context string) *NetirkError {
	if err == nil {
		return nil
	}
	
	// If it's already a NetirkError, enhance it with context
	if netirkErr, ok := err.(*NetirkError); ok {
		if netirkErr.Context == "" {
			netirkErr.Context = context
		}
		eh.logError(netirkErr)
		return netirkErr
	}
	
	// Convert regular error to NetirkError
	netirkErr := eh.classifyError(err, context)
	eh.logError(netirkErr)
	
	return netirkErr
}

// classifyError converts a regular error into a structured NetirkError
func (eh *ErrorHandler) classifyError(err error, context string) *NetirkError {
	errMsg := err.Error()
	errMsgLower := strings.ToLower(errMsg)
	
	netirkErr := &NetirkError{
		Message:   errMsg,
		Timestamp: time.Now(),
		Cause:     err,
		Context:   context,
	}
	
	// Classify error type and severity based on error message patterns
	switch {
	// Network-related errors
	case strings.Contains(errMsgLower, "connection refused"):
		netirkErr.Type = ErrorTypeNetwork
		netirkErr.Severity = ErrorSeverityHigh
		netirkErr.UserMessage = "Connection refused - the target service may be down or unreachable"
		netirkErr.Recoverable = true
		netirkErr.RetryAfter = 30 * time.Second
		
	case strings.Contains(errMsgLower, "timeout"):
		netirkErr.Type = ErrorTypeTimeout
		netirkErr.Severity = ErrorSeverityMedium
		netirkErr.UserMessage = "Request timed out - the target may be slow to respond"
		netirkErr.Recoverable = true
		netirkErr.RetryAfter = 15 * time.Second
		
	case strings.Contains(errMsgLower, "no such host") || strings.Contains(errMsgLower, "dns"):
		netirkErr.Type = ErrorTypeNetwork
		netirkErr.Severity = ErrorSeverityHigh
		netirkErr.UserMessage = "DNS resolution failed - check the target hostname"
		netirkErr.Recoverable = true
		netirkErr.RetryAfter = 60 * time.Second
		
	case strings.Contains(errMsgLower, "network unreachable"):
		netirkErr.Type = ErrorTypeNetwork
		netirkErr.Severity = ErrorSeverityCritical
		netirkErr.UserMessage = "Network unreachable - check your internet connection"
		netirkErr.Recoverable = true
		netirkErr.RetryAfter = 120 * time.Second
		
	case strings.Contains(errMsgLower, "certificate") || strings.Contains(errMsgLower, "tls") || strings.Contains(errMsgLower, "ssl"):
		netirkErr.Type = ErrorTypeSSL
		netirkErr.Severity = ErrorSeverityHigh
		netirkErr.UserMessage = "SSL/TLS certificate error - the target's certificate may be invalid or expired"
		netirkErr.Recoverable = false
		
	// Storage-related errors
	case strings.Contains(errMsgLower, "permission denied") || strings.Contains(errMsgLower, "access denied"):
		netirkErr.Type = ErrorTypeStorage
		netirkErr.Severity = ErrorSeverityHigh
		netirkErr.UserMessage = "Permission denied - check file permissions and disk space"
		netirkErr.Recoverable = false
		
	case strings.Contains(errMsgLower, "no space left") || strings.Contains(errMsgLower, "disk full"):
		netirkErr.Type = ErrorTypeStorage
		netirkErr.Severity = ErrorSeverityCritical
		netirkErr.UserMessage = "Disk full - free up disk space to continue monitoring"
		netirkErr.Recoverable = false
		
	case strings.Contains(errMsgLower, "file not found") || strings.Contains(errMsgLower, "no such file"):
		netirkErr.Type = ErrorTypeStorage
		netirkErr.Severity = ErrorSeverityMedium
		netirkErr.UserMessage = "File not found - check the file path and permissions"
		netirkErr.Recoverable = true
		netirkErr.RetryAfter = 5 * time.Second
		
	// Configuration errors
	case strings.Contains(errMsgLower, "invalid") && strings.Contains(errMsgLower, "format"):
		netirkErr.Type = ErrorTypeConfig
		netirkErr.Severity = ErrorSeverityHigh
		netirkErr.UserMessage = "Invalid configuration format - check your YAML syntax"
		netirkErr.Recoverable = false
		
	case strings.Contains(errMsgLower, "required") || strings.Contains(errMsgLower, "missing"):
		netirkErr.Type = ErrorTypeValidation
		netirkErr.Severity = ErrorSeverityHigh
		netirkErr.UserMessage = "Required configuration is missing - check your settings"
		netirkErr.Recoverable = false
		
	// Default classification
	default:
		netirkErr.Type = ErrorTypeNetwork
		netirkErr.Severity = ErrorSeverityMedium
		netirkErr.UserMessage = "An unexpected error occurred - please check the logs for details"
		netirkErr.Recoverable = true
		netirkErr.RetryAfter = 30 * time.Second
	}
	
	return netirkErr
}

// logError logs the error with appropriate formatting
func (eh *ErrorHandler) logError(err *NetirkError) {
	logLevel := "ERROR"
	switch err.Severity {
	case ErrorSeverityLow:
		logLevel = "WARN"
	case ErrorSeverityMedium:
		logLevel = "ERROR"
	case ErrorSeverityHigh:
		logLevel = "ERROR"
	case ErrorSeverityCritical:
		logLevel = "CRITICAL"
	}
	
	contextInfo := ""
	if err.Context != "" {
		contextInfo = fmt.Sprintf(" [%s]", err.Context)
	}
	
	targetInfo := ""
	if err.Target != "" {
		targetInfo = fmt.Sprintf(" (target: %s)", err.Target)
	}
	
	recoveryInfo := ""
	if err.Recoverable {
		recoveryInfo = fmt.Sprintf(" [recoverable, retry after %v]", err.GetRetryDelay())
	} else {
		recoveryInfo = " [not recoverable]"
	}
	
	eh.logger.Printf("%s: [%s:%s]%s%s %s%s", 
		logLevel, err.Type, err.Severity, contextInfo, targetInfo, err.Message, recoveryInfo)
}

// ShouldRetry determines if an operation should be retried based on error history
func (eh *ErrorHandler) ShouldRetry(key string, err *NetirkError) bool {
	if err == nil || !err.Recoverable {
		return false
	}
	
	attempts := eh.retryAttempts[key]
	if attempts >= eh.maxRetries {
		eh.logger.Printf("Max retries (%d) exceeded for %s, giving up", eh.maxRetries, key)
		return false
	}
	
	return true
}

// RecordRetryAttempt records a retry attempt for the given key
func (eh *ErrorHandler) RecordRetryAttempt(key string) {
	eh.retryAttempts[key]++
	eh.logger.Printf("Retry attempt %d/%d for %s", eh.retryAttempts[key], eh.maxRetries, key)
}

// ResetRetryCount resets the retry count for a successful operation
func (eh *ErrorHandler) ResetRetryCount(key string) {
	if _, exists := eh.retryAttempts[key]; exists {
		delete(eh.retryAttempts, key)
		eh.logger.Printf("Reset retry count for %s after successful operation", key)
	}
}

// GetRetryCount returns the current retry count for a key
func (eh *ErrorHandler) GetRetryCount(key string) int {
	return eh.retryAttempts[key]
}

// CreateNetworkError creates a network-related error
func CreateNetworkError(target string, err error, context string) *NetirkError {
	return &NetirkError{
		Type:        ErrorTypeNetwork,
		Severity:    ErrorSeverityMedium,
		Message:     err.Error(),
		UserMessage: fmt.Sprintf("Network error for %s: %s", target, err.Error()),
		Target:      target,
		Timestamp:   time.Now(),
		Cause:       err,
		Context:     context,
		Recoverable: true,
		RetryAfter:  15 * time.Second,
	}
}

// CreateStorageError creates a storage-related error
func CreateStorageError(operation string, err error) *NetirkError {
	return &NetirkError{
		Type:        ErrorTypeStorage,
		Severity:    ErrorSeverityHigh,
		Message:     err.Error(),
		UserMessage: fmt.Sprintf("Storage error during %s: %s", operation, err.Error()),
		Timestamp:   time.Now(),
		Cause:       err,
		Context:     operation,
		Recoverable: true,
		RetryAfter:  10 * time.Second,
	}
}

// CreateAlertError creates an alert-related error
func CreateAlertError(alertType string, err error) *NetirkError {
	return &NetirkError{
		Type:        ErrorTypeAlert,
		Severity:    ErrorSeverityMedium,
		Message:     err.Error(),
		UserMessage: fmt.Sprintf("Alert delivery failed for %s: %s", alertType, err.Error()),
		Timestamp:   time.Now(),
		Cause:       err,
		Context:     alertType,
		Recoverable: true,
		RetryAfter:  30 * time.Second,
	}
}

// CreateTimeoutError creates a timeout-related error
func CreateTimeoutError(operation string, timeout time.Duration) *NetirkError {
	return &NetirkError{
		Type:        ErrorTypeTimeout,
		Severity:    ErrorSeverityMedium,
		Message:     fmt.Sprintf("Operation timed out after %v", timeout),
		UserMessage: fmt.Sprintf("Operation '%s' timed out after %v - consider increasing timeout", operation, timeout),
		Timestamp:   time.Now(),
		Context:     operation,
		Recoverable: true,
		RetryAfter:  timeout / 2, // Retry after half the timeout duration
	}
}

// CreateSSLError creates an SSL-related error
func CreateSSLError(target string, err error) *NetirkError {
	return &NetirkError{
		Type:        ErrorTypeSSL,
		Severity:    ErrorSeverityHigh,
		Message:     err.Error(),
		UserMessage: fmt.Sprintf("SSL certificate error for %s: %s", target, err.Error()),
		Target:      target,
		Timestamp:   time.Now(),
		Cause:       err,
		Context:     "ssl_validation",
		Recoverable: false, // SSL errors typically require manual intervention
	}
}

// CreateValidationError creates a validation-related error
func CreateValidationError(context string, err error) *NetirkError {
	return &NetirkError{
		Type:        ErrorTypeValidation,
		Severity:    ErrorSeverityHigh,
		Message:     err.Error(),
		UserMessage: fmt.Sprintf("Validation error in %s: %s", context, err.Error()),
		Timestamp:   time.Now(),
		Cause:       err,
		Context:     context,
		Recoverable: false, // Validation errors typically require configuration fixes
	}
}

// CreateConfigError creates a configuration-related error
func CreateConfigError(context string, err error) *NetirkError {
	return &NetirkError{
		Type:        ErrorTypeConfig,
		Severity:    ErrorSeverityHigh,
		Message:     err.Error(),
		UserMessage: fmt.Sprintf("Configuration error in %s: %s", context, err.Error()),
		Timestamp:   time.Now(),
		Cause:       err,
		Context:     context,
		Recoverable: false, // Config errors typically require manual fixes
	}
}

// RetryWithBackoff executes a function with exponential backoff retry logic
func (eh *ErrorHandler) RetryWithBackoff(ctx context.Context, key string, operation func() error) error {
	var lastErr error
	
	for attempt := 0; attempt <= eh.maxRetries; attempt++ {
		if attempt > 0 {
			// Calculate exponential backoff delay
			baseDelay := 1 * time.Second
			multiplier := 1 << uint(attempt-1) // 2^(attempt-1)
			delay := time.Duration(int64(baseDelay) * int64(multiplier))
			if delay > 60*time.Second {
				delay = 60 * time.Second // Cap at 60 seconds
			}
			
			eh.logger.Printf("Retrying %s in %v (attempt %d/%d)", key, delay, attempt+1, eh.maxRetries+1)
			
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
				// Continue with retry
			}
		}
		
		err := operation()
		if err == nil {
			if attempt > 0 {
				eh.logger.Printf("Operation %s succeeded after %d retries", key, attempt)
			}
			eh.ResetRetryCount(key)
			return nil
		}
		
		lastErr = err
		netirkErr := eh.HandleError(err, key)
		
		// Don't retry if error is not recoverable
		if !netirkErr.Recoverable {
			eh.logger.Printf("Operation %s failed with non-recoverable error: %v", key, err)
			return err
		}
		
		eh.RecordRetryAttempt(key)
	}
	
	eh.logger.Printf("Operation %s failed after %d attempts: %v", key, eh.maxRetries+1, lastErr)
	return fmt.Errorf("operation failed after %d attempts: %w", eh.maxRetries+1, lastErr)
}

// WrapError wraps an existing error with additional context
func WrapError(err error, context string, target string) *NetirkError {
	if err == nil {
		return nil
	}
	
	if netirkErr, ok := err.(*NetirkError); ok {
		// Already a NetirkError, just add context
		if netirkErr.Context == "" {
			netirkErr.Context = context
		}
		if netirkErr.Target == "" {
			netirkErr.Target = target
		}
		return netirkErr
	}
	
	// Create new NetirkError
	return &NetirkError{
		Type:        ErrorTypeNetwork, // Default type
		Severity:    ErrorSeverityMedium,   // Default severity
		Message:     err.Error(),
		Target:      target,
		Timestamp:   time.Now(),
		Cause:       err,
		Context:     context,
		Recoverable: true,
		RetryAfter:  15 * time.Second,
	}
}

// IsTemporaryError checks if an error is temporary and should be retried
func IsTemporaryError(err error) bool {
	if netirkErr, ok := err.(*NetirkError); ok {
		return netirkErr.Recoverable
	}
	
	// Check for common temporary error patterns
	errMsg := strings.ToLower(err.Error())
	temporaryPatterns := []string{
		"timeout",
		"connection refused",
		"network unreachable",
		"temporary failure",
		"try again",
		"service unavailable",
	}
	
	for _, pattern := range temporaryPatterns {
		if strings.Contains(errMsg, pattern) {
			return true
		}
	}
	
	return false
}

// FormatErrorForUser formats an error message for end-user display
func FormatErrorForUser(err error) string {
	if err == nil {
		return ""
	}
	
	if netirkErr, ok := err.(*NetirkError); ok {
		return netirkErr.GetUserFriendlyMessage()
	}
	
	// Provide user-friendly messages for common error patterns
	errMsg := err.Error()
	errMsgLower := strings.ToLower(errMsg)
	
	switch {
	case strings.Contains(errMsgLower, "connection refused"):
		return "Connection refused - the target service appears to be down"
	case strings.Contains(errMsgLower, "timeout"):
		return "Request timed out - the target may be slow to respond"
	case strings.Contains(errMsgLower, "no such host"):
		return "Host not found - please check the target hostname"
	case strings.Contains(errMsgLower, "permission denied"):
		return "Permission denied - check file permissions"
	case strings.Contains(errMsgLower, "certificate"):
		return "SSL certificate error - the target's certificate may be invalid"
	default:
		return fmt.Sprintf("Error: %s", errMsg)
	}
}