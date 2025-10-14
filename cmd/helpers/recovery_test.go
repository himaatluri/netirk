package helpers

import (
	"context"
	"errors"
	"log"
	"os"
	"testing"
	"time"
)

func TestNewRecoveryManager(t *testing.T) {
	logger := log.New(os.Stdout, "TEST: ", log.LstdFlags)
	errorHandler := NewErrorHandler(logger)
	
	rm := NewRecoveryManager(errorHandler, logger)
	
	if rm == nil {
		t.Fatal("NewRecoveryManager returned nil")
	}
	
	if rm.errorHandler != errorHandler {
		t.Error("ErrorHandler not set correctly")
	}
	
	if rm.logger != logger {
		t.Error("Logger not set correctly")
	}
	
	if rm.failedTargets == nil {
		t.Error("failedTargets map not initialized")
	}
	
	if rm.alertBacklog == nil {
		t.Error("alertBacklog slice not initialized")
	}
	
	if rm.recoveryAttempts == nil {
		t.Error("recoveryAttempts map not initialized")
	}
	
	if rm.maxRecoveryTries != 5 {
		t.Errorf("Expected maxRecoveryTries to be 5, got %d", rm.maxRecoveryTries)
	}
}

func TestNewRecoveryManager_NilLogger(t *testing.T) {
	errorHandler := NewErrorHandler(nil)
	rm := NewRecoveryManager(errorHandler, nil)
	
	if rm == nil {
		t.Fatal("NewRecoveryManager returned nil")
	}
	
	if rm.logger == nil {
		t.Error("Logger should default to log.Default() when nil is passed")
	}
}

func TestRecoveryManager_HandleTargetFailure(t *testing.T) {
	rm := NewRecoveryManager(NewErrorHandler(nil), nil)
	
	target := "https://example.com"
	err := &NetirkError{
		Type:        ErrorTypeNetwork,
		Severity:    ErrorSeverityMedium,
		Message:     "connection failed",
		Recoverable: true,
		RetryAfter:  30 * time.Second,
	}
	
	// First failure
	rm.HandleTargetFailure(target, err)
	
	failureInfo, exists := rm.failedTargets[target]
	if !exists {
		t.Fatal("Target failure info not created")
	}
	
	if failureInfo.Target != target {
		t.Errorf("Expected target %s, got %s", target, failureInfo.Target)
	}
	
	if failureInfo.ConsecutiveFails != 1 {
		t.Errorf("Expected 1 consecutive failure, got %d", failureInfo.ConsecutiveFails)
	}
	
	if failureInfo.LastError != err {
		t.Error("LastError not set correctly")
	}
	
	if failureInfo.InDegradedMode {
		t.Error("Should not be in degraded mode after first failure")
	}
	
	// Second and third failures
	rm.HandleTargetFailure(target, err)
	rm.HandleTargetFailure(target, err)
	
	if failureInfo.ConsecutiveFails != 3 {
		t.Errorf("Expected 3 consecutive failures, got %d", failureInfo.ConsecutiveFails)
	}
	
	if !failureInfo.InDegradedMode {
		t.Error("Should be in degraded mode after 3 consecutive failures")
	}
	
	if !rm.degradationMode {
		t.Error("Recovery manager should be in degradation mode")
	}
}

func TestRecoveryManager_HandleTargetSuccess(t *testing.T) {
	rm := NewRecoveryManager(NewErrorHandler(nil), nil)
	
	target := "https://example.com"
	err := &NetirkError{
		Type:        ErrorTypeNetwork,
		Severity:    ErrorSeverityMedium,
		Message:     "connection failed",
		Recoverable: true,
	}
	
	// Create failure state
	rm.HandleTargetFailure(target, err)
	rm.HandleTargetFailure(target, err)
	rm.HandleTargetFailure(target, err) // Enter degraded mode
	
	// Verify failure state
	if !rm.degradationMode {
		t.Error("Should be in degradation mode before success")
	}
	
	// Handle success
	rm.HandleTargetSuccess(target)
	
	// Verify recovery
	_, exists := rm.failedTargets[target]
	if exists {
		t.Error("Target should be removed from failed targets after success")
	}
	
	if rm.degradationMode {
		t.Error("Should exit degradation mode after all targets recover")
	}
	
	_, exists = rm.recoveryAttempts[target]
	if exists {
		t.Error("Recovery attempts should be reset after success")
	}
}

func TestRecoveryManager_HandleTargetSuccess_NoFailure(t *testing.T) {
	rm := NewRecoveryManager(NewErrorHandler(nil), nil)
	
	target := "https://example.com"
	
	// Handle success for target that was never failing
	rm.HandleTargetSuccess(target)
	
	// Should not cause any issues
	if len(rm.failedTargets) != 0 {
		t.Error("Should not have any failed targets")
	}
}

func TestRecoveryManager_CalculateRetryDelay(t *testing.T) {
	rm := NewRecoveryManager(NewErrorHandler(nil), nil)
	
	tests := []struct {
		name             string
		consecutiveFails int
		errorType        ErrorType
		baseDelay        time.Duration
		expectedMin      time.Duration
		expectedMax      time.Duration
	}{
		{
			name:             "first failure network",
			consecutiveFails: 1,
			errorType:        ErrorTypeNetwork,
			baseDelay:        15 * time.Second,
			expectedMin:      15 * time.Second,
			expectedMax:      15 * time.Second,
		},
		{
			name:             "second failure network",
			consecutiveFails: 2,
			errorType:        ErrorTypeNetwork,
			baseDelay:        15 * time.Second,
			expectedMin:      30 * time.Second,
			expectedMax:      30 * time.Second,
		},
		{
			name:             "many failures network",
			consecutiveFails: 10,
			errorType:        ErrorTypeNetwork,
			baseDelay:        15 * time.Second,
			expectedMin:      4 * time.Minute, // Should be capped
			expectedMax:      5 * time.Minute,
		},
		{
			name:             "SSL error",
			consecutiveFails: 2,
			errorType:        ErrorTypeSSL,
			baseDelay:        60 * time.Second,
			expectedMin:      2 * time.Minute,
			expectedMax:      30 * time.Minute, // SSL has higher cap
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			failureInfo := &TargetFailureInfo{
				ConsecutiveFails: tt.consecutiveFails,
			}
			
			err := &NetirkError{
				Type:       tt.errorType,
				RetryAfter: tt.baseDelay,
			}
			
			delay := rm.calculateRetryDelay(failureInfo, err)
			
			if delay < tt.expectedMin || delay > tt.expectedMax {
				t.Errorf("Expected delay between %v and %v, got %v", 
					tt.expectedMin, tt.expectedMax, delay)
			}
		})
	}
}

func TestRecoveryManager_ShouldSkipTarget(t *testing.T) {
	rm := NewRecoveryManager(NewErrorHandler(nil), nil)
	
	target := "https://example.com"
	
	// Target not in failed state - should not skip
	if rm.ShouldSkipTarget(target) {
		t.Error("Should not skip healthy target")
	}
	
	// Add target to failed state with future retry time
	rm.failedTargets[target] = &TargetFailureInfo{
		Target:        target,
		NextRetryTime: time.Now().Add(1 * time.Hour),
	}
	
	// Should skip due to retry time not reached
	if !rm.ShouldSkipTarget(target) {
		t.Error("Should skip target with future retry time")
	}
	
	// Set retry time to past
	rm.failedTargets[target].NextRetryTime = time.Now().Add(-1 * time.Hour)
	
	// Should not skip
	if rm.ShouldSkipTarget(target) {
		t.Error("Should not skip target with past retry time")
	}
	
	// Put in degraded mode with max recovery attempts
	rm.failedTargets[target].InDegradedMode = true
	rm.recoveryAttempts[target] = rm.maxRecoveryTries
	
	// Should skip due to max recovery attempts
	if !rm.ShouldSkipTarget(target) {
		t.Error("Should skip target with max recovery attempts in degraded mode")
	}
}

func TestRecoveryManager_GetTargetStatus(t *testing.T) {
	rm := NewRecoveryManager(NewErrorHandler(nil), nil)
	
	target := "https://example.com"
	
	// Healthy target
	status := rm.GetTargetStatus(target)
	if status != "healthy" {
		t.Errorf("Expected 'healthy', got %s", status)
	}
	
	// Failing target
	rm.failedTargets[target] = &TargetFailureInfo{
		Target:        target,
		NextRetryTime: time.Now().Add(-1 * time.Hour),
	}
	
	status = rm.GetTargetStatus(target)
	if status != "failing" {
		t.Errorf("Expected 'failing', got %s", status)
	}
	
	// Backing off target
	rm.failedTargets[target].NextRetryTime = time.Now().Add(1 * time.Hour)
	
	status = rm.GetTargetStatus(target)
	if status != "backing_off" {
		t.Errorf("Expected 'backing_off', got %s", status)
	}
	
	// Degraded target
	rm.failedTargets[target].InDegradedMode = true
	rm.failedTargets[target].NextRetryTime = time.Now().Add(-1 * time.Hour)
	
	status = rm.GetTargetStatus(target)
	if status != "degraded" {
		t.Errorf("Expected 'degraded', got %s", status)
	}
}

func TestRecoveryManager_RecoverStorageOperation(t *testing.T) {
	rm := NewRecoveryManager(NewErrorHandler(nil), nil)
	
	// Test successful operation
	callCount := 0
	err := rm.RecoverStorageOperation(func() error {
		callCount++
		return nil
	}, "test_operation")
	
	if err != nil {
		t.Errorf("Expected no error for successful operation, got %v", err)
	}
	
	if callCount != 1 {
		t.Errorf("Expected 1 call for successful operation, got %d", callCount)
	}
	
	// Test operation that eventually succeeds
	callCount = 0
	err = rm.RecoverStorageOperation(func() error {
		callCount++
		if callCount == 1 {
			return errors.New("temporary failure")
		}
		return nil
	}, "test_operation")
	
	if err != nil {
		t.Errorf("Expected no error for eventually successful operation, got %v", err)
	}
	
	if callCount < 2 {
		t.Errorf("Expected at least 2 calls for eventually successful operation, got %d", callCount)
	}
}

func TestRecoveryManager_HandleStorageError(t *testing.T) {
	rm := NewRecoveryManager(NewErrorHandler(nil), nil)
	
	tests := []struct {
		name      string
		err       error
		operation string
		expectNil bool
	}{
		{
			name:      "permission denied",
			err:       errors.New("permission denied"),
			operation: "save_data",
			expectNil: true, // Should be handled
		},
		{
			name:      "no space left",
			err:       errors.New("no space left on device"),
			operation: "save_data",
			expectNil: false, // Returns error indicating manual intervention needed
		},
		{
			name:      "file not found",
			err:       errors.New("file not found"),
			operation: "save_data",
			expectNil: true, // Should be handled
		},
		{
			name:      "unknown error",
			err:       errors.New("unknown storage error"),
			operation: "save_data",
			expectNil: false, // Should return original error
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := rm.handleStorageError(tt.err, tt.operation)
			
			if tt.expectNil && result != nil {
				t.Errorf("Expected nil error, got %v", result)
			}
			
			if !tt.expectNil && result == nil {
				t.Error("Expected non-nil error")
			}
		})
	}
}

func TestRecoveryManager_RecoverAlertDelivery(t *testing.T) {
	rm := NewRecoveryManager(NewErrorHandler(nil), nil)
	
	alert := &Alert{
		Type:      AlertTypeFailure,
		Target:    "https://example.com",
		Message:   "Test alert",
		Timestamp: time.Now(),
	}
	
	// Test successful delivery
	callCount := 0
	err := rm.RecoverAlertDelivery(alert, func(a *Alert) error {
		callCount++
		return nil
	})
	
	if err != nil {
		t.Errorf("Expected no error for successful delivery, got %v", err)
	}
	
	if callCount != 1 {
		t.Errorf("Expected 1 call for successful delivery, got %d", callCount)
	}
	
	// Alert should be removed from backlog
	if len(rm.alertBacklog) != 0 {
		t.Errorf("Expected empty alert backlog after successful delivery, got %d", len(rm.alertBacklog))
	}
	
	// Test failed delivery
	callCount = 0
	err = rm.RecoverAlertDelivery(alert, func(a *Alert) error {
		callCount++
		return errors.New("delivery failed")
	})
	
	if err == nil {
		t.Error("Expected error for failed delivery")
	}
	
	// Alert should remain in backlog
	if len(rm.alertBacklog) == 0 {
		t.Error("Expected alert to remain in backlog after failed delivery")
	}
}

func TestRecoveryManager_ProcessAlertBacklog(t *testing.T) {
	rm := NewRecoveryManager(NewErrorHandler(nil), nil)
	
	// Add alerts to backlog
	alert1 := &Alert{
		Type:      AlertTypeFailure,
		Target:    "https://example.com",
		Message:   "Test alert 1",
		Timestamp: time.Now(),
	}
	
	alert2 := &Alert{
		Type:      AlertTypeRecovery,
		Target:    "https://test.com",
		Message:   "Test alert 2",
		Timestamp: time.Now(),
	}
	
	// Old alert (should be discarded)
	oldAlert := &Alert{
		Type:      AlertTypeFailure,
		Target:    "https://old.com",
		Message:   "Old alert",
		Timestamp: time.Now().Add(-25 * time.Hour),
	}
	
	rm.alertBacklog = []*Alert{alert1, alert2, oldAlert}
	
	deliveredAlerts := make([]*Alert, 0)
	rm.ProcessAlertBacklog(func(a *Alert) error {
		if a.Target == "https://example.com" {
			deliveredAlerts = append(deliveredAlerts, a)
			return nil // Successful delivery
		}
		return errors.New("delivery failed") // Failed delivery
	})
	
	// Check that successful alert was delivered and removed
	if len(deliveredAlerts) != 1 {
		t.Errorf("Expected 1 delivered alert, got %d", len(deliveredAlerts))
	}
	
	// Check that old alert was discarded
	foundOldAlert := false
	for _, alert := range rm.alertBacklog {
		if alert.Target == "https://old.com" {
			foundOldAlert = true
			break
		}
	}
	if foundOldAlert {
		t.Error("Old alert should have been discarded")
	}
	
	// Check that failed alert remains in backlog
	foundFailedAlert := false
	for _, alert := range rm.alertBacklog {
		if alert.Target == "https://test.com" {
			foundFailedAlert = true
			break
		}
	}
	if !foundFailedAlert {
		t.Error("Failed alert should remain in backlog")
	}
}

func TestRecoveryManager_GetRecoveryStats(t *testing.T) {
	rm := NewRecoveryManager(NewErrorHandler(nil), nil)
	
	// Add some test data
	rm.degradationMode = true
	rm.failedTargets["target1"] = &TargetFailureInfo{
		Target:         "target1",
		InDegradedMode: true,
	}
	rm.failedTargets["target2"] = &TargetFailureInfo{
		Target:         "target2",
		InDegradedMode: false,
	}
	rm.alertBacklog = []*Alert{{}, {}} // 2 alerts
	rm.storageBackups = []string{"backup1", "backup2"}
	rm.recoveryAttempts["target1"] = 3
	rm.recoveryAttempts["target2"] = 2
	
	stats := rm.GetRecoveryStats()
	
	if stats["degradation_mode"] != true {
		t.Error("Expected degradation_mode to be true")
	}
	
	if stats["failed_targets_count"] != 2 {
		t.Errorf("Expected failed_targets_count to be 2, got %v", stats["failed_targets_count"])
	}
	
	if stats["alert_backlog_size"] != 2 {
		t.Errorf("Expected alert_backlog_size to be 2, got %v", stats["alert_backlog_size"])
	}
	
	if stats["storage_backups_count"] != 2 {
		t.Errorf("Expected storage_backups_count to be 2, got %v", stats["storage_backups_count"])
	}
	
	if stats["total_recovery_attempts"] != 5 {
		t.Errorf("Expected total_recovery_attempts to be 5, got %v", stats["total_recovery_attempts"])
	}
	
	statusCounts, ok := stats["target_status_counts"].(map[string]int)
	if !ok {
		t.Fatal("Expected target_status_counts to be map[string]int")
	}
	
	if statusCounts["degraded"] != 1 {
		t.Errorf("Expected 1 degraded target, got %d", statusCounts["degraded"])
	}
	
	if statusCounts["failing"] != 1 {
		t.Errorf("Expected 1 failing target, got %d", statusCounts["failing"])
	}
}

func TestRecoveryManager_StateQueries(t *testing.T) {
	rm := NewRecoveryManager(NewErrorHandler(nil), nil)
	
	// Initial state
	if rm.IsInDegradationMode() {
		t.Error("Should not be in degradation mode initially")
	}
	
	if len(rm.GetFailedTargets()) != 0 {
		t.Error("Should have no failed targets initially")
	}
	
	if rm.GetAlertBacklogSize() != 0 {
		t.Error("Should have empty alert backlog initially")
	}
	
	// Add failed target
	rm.failedTargets["target1"] = &TargetFailureInfo{
		Target:         "target1",
		InDegradedMode: true,
	}
	rm.degradationMode = true
	
	// Add alert to backlog
	rm.alertBacklog = []*Alert{{}}
	
	// Check updated state
	if !rm.IsInDegradationMode() {
		t.Error("Should be in degradation mode")
	}
	
	failedTargets := rm.GetFailedTargets()
	if len(failedTargets) != 1 || failedTargets[0] != "target1" {
		t.Errorf("Expected ['target1'], got %v", failedTargets)
	}
	
	if rm.GetAlertBacklogSize() != 1 {
		t.Error("Should have 1 alert in backlog")
	}
}

func TestRecoveryManager_Reset(t *testing.T) {
	rm := NewRecoveryManager(NewErrorHandler(nil), nil)
	
	// Add some state
	rm.failedTargets["target1"] = &TargetFailureInfo{}
	rm.alertBacklog = []*Alert{{}}
	rm.recoveryAttempts["target1"] = 3
	rm.degradationMode = true
	
	// Reset
	rm.Reset()
	
	// Verify state is cleared
	if len(rm.failedTargets) != 0 {
		t.Error("Failed targets should be cleared after reset")
	}
	
	if len(rm.alertBacklog) != 0 {
		t.Error("Alert backlog should be cleared after reset")
	}
	
	if len(rm.recoveryAttempts) != 0 {
		t.Error("Recovery attempts should be cleared after reset")
	}
	
	if rm.degradationMode {
		t.Error("Degradation mode should be false after reset")
	}
}

func TestRecoveryManager_StartRecoveryLoop(t *testing.T) {
	rm := NewRecoveryManager(NewErrorHandler(nil), nil)
	
	// Add a target ready for retry
	rm.failedTargets["target1"] = &TargetFailureInfo{
		Target:        "target1",
		NextRetryTime: time.Now().Add(-1 * time.Hour), // Past retry time
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	
	// Start recovery loop (should exit when context is cancelled)
	rm.StartRecoveryLoop(ctx, 50*time.Millisecond)
	
	// Check that recovery attempt was recorded
	if rm.recoveryAttempts["target1"] == 0 {
		t.Error("Expected recovery attempt to be recorded")
	}
}

func TestRecoveryManager_CheckGlobalDegradationMode(t *testing.T) {
	rm := NewRecoveryManager(NewErrorHandler(nil), nil)
	
	// Set degradation mode
	rm.degradationMode = true
	
	// Add targets, some in degraded mode
	rm.failedTargets["target1"] = &TargetFailureInfo{
		InDegradedMode: true,
	}
	rm.failedTargets["target2"] = &TargetFailureInfo{
		InDegradedMode: false,
	}
	
	// Should remain in degradation mode
	rm.checkGlobalDegradationMode()
	if !rm.degradationMode {
		t.Error("Should remain in degradation mode with degraded targets")
	}
	
	// Remove degraded target
	rm.failedTargets["target1"].InDegradedMode = false
	
	// Should exit degradation mode
	rm.checkGlobalDegradationMode()
	if rm.degradationMode {
		t.Error("Should exit degradation mode when no targets are degraded")
	}
}

func TestRecoveryManager_RemoveFromAlertBacklog(t *testing.T) {
	rm := NewRecoveryManager(NewErrorHandler(nil), nil)
	
	timestamp := time.Now()
	alert1 := &Alert{
		Type:      AlertTypeFailure,
		Target:    "target1",
		Timestamp: timestamp,
	}
	alert2 := &Alert{
		Type:      AlertTypeRecovery,
		Target:    "target2",
		Timestamp: timestamp,
	}
	
	rm.alertBacklog = []*Alert{alert1, alert2}
	
	// Remove first alert
	rm.removeFromAlertBacklog(alert1)
	
	if len(rm.alertBacklog) != 1 {
		t.Errorf("Expected 1 alert in backlog after removal, got %d", len(rm.alertBacklog))
	}
	
	if rm.alertBacklog[0] != alert2 {
		t.Error("Wrong alert removed from backlog")
	}
	
	// Try to remove non-existent alert
	nonExistentAlert := &Alert{
		Type:      AlertTypeSSL,
		Target:    "target3",
		Timestamp: timestamp,
	}
	
	rm.removeFromAlertBacklog(nonExistentAlert)
	
	if len(rm.alertBacklog) != 1 {
		t.Error("Backlog size should not change when removing non-existent alert")
	}
}

// Helper function tests
func TestContainsFunction(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		substr   string
		expected bool
	}{
		{"exact match", "hello", "hello", true},
		{"substring at start", "hello world", "hello", true},
		{"substring at end", "hello world", "world", true},
		{"substring in middle", "hello world", "lo wo", true},
		{"not found", "hello world", "xyz", false},
		{"empty substring", "hello", "", true},
		{"empty string", "", "hello", false},
		{"both empty", "", "", true},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := contains(tt.s, tt.substr)
			if result != tt.expected {
				t.Errorf("contains(%q, %q) = %v, expected %v", tt.s, tt.substr, result, tt.expected)
			}
		})
	}
}

func TestContainsSubstring(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		substr   string
		expected bool
	}{
		{"found at start", "hello world", "hello", true},
		{"found at end", "hello world", "world", true},
		{"found in middle", "hello world", "lo wo", true},
		{"not found", "hello world", "xyz", false},
		{"empty substring", "hello", "", true},
		{"substring longer than string", "hi", "hello", false},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := containsSubstring(tt.s, tt.substr)
			if result != tt.expected {
				t.Errorf("containsSubstring(%q, %q) = %v, expected %v", tt.s, tt.substr, result, tt.expected)
			}
		})
	}
}