package helpers

import (
	"context"
	"errors"
	"log"
	"os"
	"strings"
	"testing"
	"time"
)

func TestNewErrorHandler(t *testing.T) {
	logger := log.New(os.Stdout, "TEST: ", log.LstdFlags)
	eh := NewErrorHandler(logger)
	
	if eh == nil {
		t.Fatal("NewErrorHandler returned nil")
	}
	
	if eh.logger != logger {
		t.Error("Logger not set correctly")
	}
	
	if eh.maxRetries != 3 {
		t.Errorf("Expected default maxRetries to be 3, got %d", eh.maxRetries)
	}
	
	if eh.retryAttempts == nil {
		t.Error("retryAttempts map not initialized")
	}
	
	if eh.lastErrors == nil {
		t.Error("lastErrors map not initialized")
	}
}

func TestNewErrorHandler_NilLogger(t *testing.T) {
	eh := NewErrorHandler(nil)
	
	if eh == nil {
		t.Fatal("NewErrorHandler returned nil")
	}
	
	if eh.logger == nil {
		t.Error("Logger should default to log.Default() when nil is passed")
	}
}

func TestNetirkError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *NetirkError
		expected string
	}{
		{
			name: "error with target",
			err: &NetirkError{
				Type:     ErrorTypeNetwork,
				Severity: ErrorSeverityHigh,
				Message:  "connection failed",
				Target:   "https://example.com",
			},
			expected: "[network:high] connection failed (target: https://example.com)",
		},
		{
			name: "error without target",
			err: &NetirkError{
				Type:     ErrorTypeStorage,
				Severity: ErrorSeverityMedium,
				Message:  "disk full",
			},
			expected: "[storage:medium] disk full",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.err.Error()
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestNetirkError_Unwrap(t *testing.T) {
	originalErr := errors.New("original error")
	netirkErr := &NetirkError{
		Type:    ErrorTypeNetwork,
		Message: "wrapped error",
		Cause:   originalErr,
	}
	
	unwrapped := netirkErr.Unwrap()
	if unwrapped != originalErr {
		t.Errorf("Expected unwrapped error to be %v, got %v", originalErr, unwrapped)
	}
}

func TestNetirkError_GetUserFriendlyMessage(t *testing.T) {
	tests := []struct {
		name     string
		err      *NetirkError
		expected string
	}{
		{
			name: "with user message",
			err: &NetirkError{
				Message:     "technical error",
				UserMessage: "user friendly message",
			},
			expected: "user friendly message",
		},
		{
			name: "without user message",
			err: &NetirkError{
				Message: "technical error",
			},
			expected: "technical error",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.err.GetUserFriendlyMessage()
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestNetirkError_GetRetryDelay(t *testing.T) {
	tests := []struct {
		name     string
		err      *NetirkError
		expected time.Duration
	}{
		{
			name: "with explicit retry after",
			err: &NetirkError{
				Type:       ErrorTypeNetwork,
				Severity:   ErrorSeverityMedium,
				RetryAfter: 45 * time.Second,
			},
			expected: 45 * time.Second,
		},
		{
			name: "network error low severity",
			err: &NetirkError{
				Type:     ErrorTypeNetwork,
				Severity: ErrorSeverityLow,
			},
			expected: 5 * time.Second,
		},
		{
			name: "network error critical severity",
			err: &NetirkError{
				Type:     ErrorTypeNetwork,
				Severity: ErrorSeverityCritical,
			},
			expected: 60 * time.Second,
		},
		{
			name: "storage error",
			err: &NetirkError{
				Type:     ErrorTypeStorage,
				Severity: ErrorSeverityMedium,
			},
			expected: 10 * time.Second,
		},
		{
			name: "unknown error type",
			err: &NetirkError{
				Type:     "unknown",
				Severity: ErrorSeverityMedium,
			},
			expected: 15 * time.Second,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.err.GetRetryDelay()
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestErrorHandler_ClassifyError(t *testing.T) {
	eh := NewErrorHandler(nil)
	
	tests := []struct {
		name           string
		err            error
		context        string
		expectedType   ErrorType
		expectedSeverity ErrorSeverity
		expectedRecoverable bool
	}{
		{
			name:           "connection refused",
			err:            errors.New("connection refused"),
			context:        "test",
			expectedType:   ErrorTypeNetwork,
			expectedSeverity: ErrorSeverityHigh,
			expectedRecoverable: true,
		},
		{
			name:           "timeout error",
			err:            errors.New("request timeout"),
			context:        "test",
			expectedType:   ErrorTypeTimeout,
			expectedSeverity: ErrorSeverityMedium,
			expectedRecoverable: true,
		},
		{
			name:           "DNS error",
			err:            errors.New("no such host"),
			context:        "test",
			expectedType:   ErrorTypeNetwork,
			expectedSeverity: ErrorSeverityHigh,
			expectedRecoverable: true,
		},
		{
			name:           "SSL error",
			err:            errors.New("certificate expired"),
			context:        "test",
			expectedType:   ErrorTypeSSL,
			expectedSeverity: ErrorSeverityHigh,
			expectedRecoverable: false,
		},
		{
			name:           "permission denied",
			err:            errors.New("permission denied"),
			context:        "test",
			expectedType:   ErrorTypeStorage,
			expectedSeverity: ErrorSeverityHigh,
			expectedRecoverable: false,
		},
		{
			name:           "disk full",
			err:            errors.New("no space left on device"),
			context:        "test",
			expectedType:   ErrorTypeStorage,
			expectedSeverity: ErrorSeverityCritical,
			expectedRecoverable: false,
		},
		{
			name:           "file not found",
			err:            errors.New("file not found"),
			context:        "test",
			expectedType:   ErrorTypeStorage,
			expectedSeverity: ErrorSeverityMedium,
			expectedRecoverable: true,
		},
		{
			name:           "invalid format",
			err:            errors.New("invalid format"),
			context:        "test",
			expectedType:   ErrorTypeConfig,
			expectedSeverity: ErrorSeverityHigh,
			expectedRecoverable: false,
		},
		{
			name:           "required field missing",
			err:            errors.New("required field missing"),
			context:        "test",
			expectedType:   ErrorTypeValidation,
			expectedSeverity: ErrorSeverityHigh,
			expectedRecoverable: false,
		},
		{
			name:           "unknown error",
			err:            errors.New("unknown error"),
			context:        "test",
			expectedType:   ErrorTypeNetwork,
			expectedSeverity: ErrorSeverityMedium,
			expectedRecoverable: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := eh.classifyError(tt.err, tt.context)
			
			if result.Type != tt.expectedType {
				t.Errorf("Expected type %s, got %s", tt.expectedType, result.Type)
			}
			
			if result.Severity != tt.expectedSeverity {
				t.Errorf("Expected severity %s, got %s", tt.expectedSeverity, result.Severity)
			}
			
			if result.Recoverable != tt.expectedRecoverable {
				t.Errorf("Expected recoverable %v, got %v", tt.expectedRecoverable, result.Recoverable)
			}
			
			if result.Context != tt.context {
				t.Errorf("Expected context %s, got %s", tt.context, result.Context)
			}
			
			if result.Cause != tt.err {
				t.Errorf("Expected cause to be original error")
			}
		})
	}
}

func TestErrorHandler_HandleError(t *testing.T) {
	eh := NewErrorHandler(nil)
	
	// Test with regular error
	originalErr := errors.New("test error")
	result := eh.HandleError(originalErr, "test_context")
	
	if result == nil {
		t.Fatal("HandleError returned nil")
	}
	
	if result.Context != "test_context" {
		t.Errorf("Expected context 'test_context', got %s", result.Context)
	}
	
	if result.Cause != originalErr {
		t.Error("Expected cause to be original error")
	}
	
	// Test with nil error
	result = eh.HandleError(nil, "test_context")
	if result != nil {
		t.Error("Expected nil result for nil error")
	}
	
	// Test with existing NetirkError
	netirkErr := &NetirkError{
		Type:    ErrorTypeNetwork,
		Message: "existing error",
	}
	result = eh.HandleError(netirkErr, "new_context")
	
	if result != netirkErr {
		t.Error("Expected same NetirkError instance")
	}
	
	if result.Context != "new_context" {
		t.Errorf("Expected context to be updated to 'new_context', got %s", result.Context)
	}
}

func TestErrorHandler_ShouldRetry(t *testing.T) {
	eh := NewErrorHandler(nil)
	eh.SetMaxRetries(2)
	
	recoverableErr := &NetirkError{
		Type:        ErrorTypeNetwork,
		Recoverable: true,
	}
	
	nonRecoverableErr := &NetirkError{
		Type:        ErrorTypeSSL,
		Recoverable: false,
	}
	
	// Test with recoverable error, no previous attempts
	if !eh.ShouldRetry("test_key", recoverableErr) {
		t.Error("Should retry recoverable error with no previous attempts")
	}
	
	// Test with non-recoverable error
	if eh.ShouldRetry("test_key", nonRecoverableErr) {
		t.Error("Should not retry non-recoverable error")
	}
	
	// Test with nil error
	if eh.ShouldRetry("test_key", nil) {
		t.Error("Should not retry nil error")
	}
	
	// Test max retries exceeded
	eh.retryAttempts["test_key"] = 3 // Exceed max retries
	if eh.ShouldRetry("test_key", recoverableErr) {
		t.Error("Should not retry when max retries exceeded")
	}
}

func TestErrorHandler_RetryOperations(t *testing.T) {
	eh := NewErrorHandler(nil)
	eh.SetMaxRetries(1)
	
	// Test successful operation
	callCount := 0
	err := eh.RetryWithBackoff(context.Background(), "test_op", func() error {
		callCount++
		return nil
	})
	
	if err != nil {
		t.Errorf("Expected no error for successful operation, got %v", err)
	}
	
	if callCount != 1 {
		t.Errorf("Expected 1 call for successful operation, got %d", callCount)
	}
	
	// Test operation that fails then succeeds
	callCount = 0
	err = eh.RetryWithBackoff(context.Background(), "test_op2", func() error {
		callCount++
		if callCount == 1 {
			return errors.New("temporary failure")
		}
		return nil
	})
	
	if err != nil {
		t.Errorf("Expected no error for eventually successful operation, got %v", err)
	}
	
	if callCount != 2 {
		t.Errorf("Expected 2 calls for eventually successful operation, got %d", callCount)
	}
	
	// Test operation that always fails
	callCount = 0
	err = eh.RetryWithBackoff(context.Background(), "test_op3", func() error {
		callCount++
		return errors.New("permanent failure")
	})
	
	if err == nil {
		t.Error("Expected error for always failing operation")
	}
	
	if callCount != 2 { // 1 initial + 1 retry
		t.Errorf("Expected 2 calls for always failing operation, got %d", callCount)
	}
}

func TestErrorHandler_RetryWithContext(t *testing.T) {
	eh := NewErrorHandler(nil)
	eh.SetMaxRetries(5)
	
	// Test context cancellation
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	
	callCount := 0
	err := eh.RetryWithBackoff(ctx, "test_op", func() error {
		callCount++
		time.Sleep(100 * time.Millisecond) // Longer than context timeout
		return errors.New("slow operation")
	})
	
	if err != context.DeadlineExceeded {
		t.Errorf("Expected context deadline exceeded, got %v", err)
	}
	
	if callCount != 1 {
		t.Errorf("Expected 1 call before context cancellation, got %d", callCount)
	}
}

func TestCreateErrorFunctions(t *testing.T) {
	originalErr := errors.New("original error")
	
	// Test CreateNetworkError
	netErr := CreateNetworkError("https://example.com", originalErr, "test_context")
	if netErr.Type != ErrorTypeNetwork {
		t.Errorf("Expected network error type, got %s", netErr.Type)
	}
	if netErr.Target != "https://example.com" {
		t.Errorf("Expected target 'https://example.com', got %s", netErr.Target)
	}
	if netErr.Cause != originalErr {
		t.Error("Expected cause to be original error")
	}
	
	// Test CreateStorageError
	storageErr := CreateStorageError("save_operation", originalErr)
	if storageErr.Type != ErrorTypeStorage {
		t.Errorf("Expected storage error type, got %s", storageErr.Type)
	}
	if storageErr.Context != "save_operation" {
		t.Errorf("Expected context 'save_operation', got %s", storageErr.Context)
	}
	
	// Test CreateAlertError
	alertErr := CreateAlertError("webhook", originalErr)
	if alertErr.Type != ErrorTypeAlert {
		t.Errorf("Expected alert error type, got %s", alertErr.Type)
	}
	
	// Test CreateTimeoutError
	timeoutErr := CreateTimeoutError("http_request", 30*time.Second)
	if timeoutErr.Type != ErrorTypeTimeout {
		t.Errorf("Expected timeout error type, got %s", timeoutErr.Type)
	}
	if !strings.Contains(timeoutErr.Message, "30s") {
		t.Error("Expected timeout duration in message")
	}
	
	// Test CreateSSLError
	sslErr := CreateSSLError("https://example.com", originalErr)
	if sslErr.Type != ErrorTypeSSL {
		t.Errorf("Expected SSL error type, got %s", sslErr.Type)
	}
	if sslErr.Recoverable {
		t.Error("Expected SSL error to be non-recoverable")
	}
	
	// Test CreateValidationError
	validationErr := CreateValidationError("config_validation", originalErr)
	if validationErr.Type != ErrorTypeValidation {
		t.Errorf("Expected validation error type, got %s", validationErr.Type)
	}
	if validationErr.Recoverable {
		t.Error("Expected validation error to be non-recoverable")
	}
	
	// Test CreateConfigError
	configErr := CreateConfigError("yaml_parsing", originalErr)
	if configErr.Type != ErrorTypeConfig {
		t.Errorf("Expected config error type, got %s", configErr.Type)
	}
	if configErr.Recoverable {
		t.Error("Expected config error to be non-recoverable")
	}
}

func TestWrapError(t *testing.T) {
	originalErr := errors.New("original error")
	
	// Test wrapping regular error
	wrapped := WrapError(originalErr, "test_context", "https://example.com")
	if wrapped == nil {
		t.Fatal("WrapError returned nil")
	}
	if wrapped.Context != "test_context" {
		t.Errorf("Expected context 'test_context', got %s", wrapped.Context)
	}
	if wrapped.Target != "https://example.com" {
		t.Errorf("Expected target 'https://example.com', got %s", wrapped.Target)
	}
	if wrapped.Cause != originalErr {
		t.Error("Expected cause to be original error")
	}
	
	// Test wrapping NetirkError
	netirkErr := &NetirkError{
		Type:    ErrorTypeNetwork,
		Message: "existing error",
	}
	wrapped = WrapError(netirkErr, "new_context", "new_target")
	if wrapped != netirkErr {
		t.Error("Expected same NetirkError instance")
	}
	if wrapped.Context != "new_context" {
		t.Errorf("Expected context to be updated to 'new_context', got %s", wrapped.Context)
	}
	if wrapped.Target != "new_target" {
		t.Errorf("Expected target to be updated to 'new_target', got %s", wrapped.Target)
	}
	
	// Test wrapping nil error
	wrapped = WrapError(nil, "context", "target")
	if wrapped != nil {
		t.Error("Expected nil result for nil error")
	}
}

func TestIsTemporaryError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name: "recoverable NetirkError",
			err: &NetirkError{
				Type:        ErrorTypeNetwork,
				Recoverable: true,
			},
			expected: true,
		},
		{
			name: "non-recoverable NetirkError",
			err: &NetirkError{
				Type:        ErrorTypeSSL,
				Recoverable: false,
			},
			expected: false,
		},
		{
			name:     "timeout error",
			err:      errors.New("request timeout"),
			expected: true,
		},
		{
			name:     "connection refused",
			err:      errors.New("connection refused"),
			expected: true,
		},
		{
			name:     "network unreachable",
			err:      errors.New("network unreachable"),
			expected: true,
		},
		{
			name:     "service unavailable",
			err:      errors.New("service unavailable"),
			expected: true,
		},
		{
			name:     "permanent error",
			err:      errors.New("invalid configuration"),
			expected: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsTemporaryError(tt.err)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestFormatErrorForUser(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: "",
		},
		{
			name: "NetirkError with user message",
			err: &NetirkError{
				Message:     "technical error",
				UserMessage: "user friendly message",
			},
			expected: "user friendly message",
		},
		{
			name:     "connection refused",
			err:      errors.New("connection refused"),
			expected: "Connection refused - the target service appears to be down",
		},
		{
			name:     "timeout error",
			err:      errors.New("request timeout"),
			expected: "Request timed out - the target may be slow to respond",
		},
		{
			name:     "DNS error",
			err:      errors.New("no such host"),
			expected: "Host not found - please check the target hostname",
		},
		{
			name:     "permission error",
			err:      errors.New("permission denied"),
			expected: "Permission denied - check file permissions",
		},
		{
			name:     "certificate error",
			err:      errors.New("certificate expired"),
			expected: "SSL certificate error - the target's certificate may be invalid",
		},
		{
			name:     "unknown error",
			err:      errors.New("unknown error"),
			expected: "Error: unknown error",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatErrorForUser(tt.err)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestErrorHandler_SetMaxRetries(t *testing.T) {
	eh := NewErrorHandler(nil)
	
	eh.SetMaxRetries(5)
	if eh.maxRetries != 5 {
		t.Errorf("Expected maxRetries to be 5, got %d", eh.maxRetries)
	}
}

func TestErrorHandler_RecordAndResetRetryCount(t *testing.T) {
	eh := NewErrorHandler(nil)
	
	key := "test_key"
	
	// Initial count should be 0
	if count := eh.GetRetryCount(key); count != 0 {
		t.Errorf("Expected initial retry count to be 0, got %d", count)
	}
	
	// Record attempts
	eh.RecordRetryAttempt(key)
	if count := eh.GetRetryCount(key); count != 1 {
		t.Errorf("Expected retry count to be 1, got %d", count)
	}
	
	eh.RecordRetryAttempt(key)
	if count := eh.GetRetryCount(key); count != 2 {
		t.Errorf("Expected retry count to be 2, got %d", count)
	}
	
	// Reset count
	eh.ResetRetryCount(key)
	if count := eh.GetRetryCount(key); count != 0 {
		t.Errorf("Expected retry count to be 0 after reset, got %d", count)
	}
}