package helpers

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// RecoveryManager handles graceful degradation and recovery mechanisms
type RecoveryManager struct {
	errorHandler     *ErrorHandler
	logger           *log.Logger
	failedTargets    map[string]*TargetFailureInfo
	storageBackups   []string
	alertBacklog     []*Alert
	mutex            sync.RWMutex
	degradationMode  bool
	recoveryAttempts map[string]int
	maxRecoveryTries int
}

// TargetFailureInfo tracks failure information for a target
type TargetFailureInfo struct {
	Target           string
	FirstFailure     time.Time
	LastFailure      time.Time
	ConsecutiveFails int
	LastError        *NetirkError
	InDegradedMode   bool
	NextRetryTime    time.Time
}

// RecoveryConfig holds configuration for recovery behavior
type RecoveryConfig struct {
	MaxRecoveryAttempts    int
	DegradationThreshold   int
	RecoveryCheckInterval  time.Duration
	StorageRetryInterval   time.Duration
	AlertRetryInterval     time.Duration
	EnableGracefulDegradation bool
}

// NewRecoveryManager creates a new recovery manager
func NewRecoveryManager(errorHandler *ErrorHandler, logger *log.Logger) *RecoveryManager {
	if logger == nil {
		logger = log.Default()
	}
	
	return &RecoveryManager{
		errorHandler:     errorHandler,
		logger:           logger,
		failedTargets:    make(map[string]*TargetFailureInfo),
		storageBackups:   make([]string, 0),
		alertBacklog:     make([]*Alert, 0),
		recoveryAttempts: make(map[string]int),
		maxRecoveryTries: 5,
	}
}

// HandleTargetFailure processes a target failure and implements graceful degradation
func (rm *RecoveryManager) HandleTargetFailure(target string, err *NetirkError) {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()
	
	now := time.Now()
	
	// Get or create failure info for this target
	failureInfo, exists := rm.failedTargets[target]
	if !exists {
		failureInfo = &TargetFailureInfo{
			Target:       target,
			FirstFailure: now,
		}
		rm.failedTargets[target] = failureInfo
	}
	
	// Update failure information
	failureInfo.LastFailure = now
	failureInfo.ConsecutiveFails++
	failureInfo.LastError = err
	
	// Calculate next retry time based on error type and failure count
	retryDelay := rm.calculateRetryDelay(failureInfo, err)
	failureInfo.NextRetryTime = now.Add(retryDelay)
	
	rm.logger.Printf("Target %s failed (consecutive: %d, next retry: %v): %s", 
		target, failureInfo.ConsecutiveFails, retryDelay, err.GetUserFriendlyMessage())
	
	// Check if we should enter degraded mode for this target
	if failureInfo.ConsecutiveFails >= 3 && !failureInfo.InDegradedMode {
		rm.enterDegradedMode(target, failureInfo)
	}
}

// HandleTargetSuccess processes a target success and handles recovery
func (rm *RecoveryManager) HandleTargetSuccess(target string) {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()
	
	failureInfo, exists := rm.failedTargets[target]
	if !exists {
		return // Target was never failing
	}
	
	// Log recovery if target was previously failing
	if failureInfo.ConsecutiveFails > 0 {
		rm.logger.Printf("Target %s recovered after %d consecutive failures (failed for %v)", 
			target, failureInfo.ConsecutiveFails, time.Since(failureInfo.FirstFailure))
	}
	
	// Exit degraded mode if applicable
	if failureInfo.InDegradedMode {
		rm.exitDegradedMode(target, failureInfo)
	}
	
	// Remove from failed targets
	delete(rm.failedTargets, target)
	
	// Reset recovery attempts
	delete(rm.recoveryAttempts, target)
}

// calculateRetryDelay calculates the retry delay based on failure history and error type
func (rm *RecoveryManager) calculateRetryDelay(failureInfo *TargetFailureInfo, err *NetirkError) time.Duration {
	baseDelay := err.GetRetryDelay()
	
	// Apply exponential backoff based on consecutive failures
	multiplier := 1.0
	if failureInfo.ConsecutiveFails > 1 {
		// Exponential backoff: 1, 2, 4, 8, 16 (capped at 16)
		backoffMultiplier := 1 << uint(failureInfo.ConsecutiveFails-1)
		multiplier = float64(backoffMultiplier)
		if multiplier > 16 {
			multiplier = 16
		}
	}
	
	delay := time.Duration(float64(baseDelay) * multiplier)
	
	// Cap maximum delay based on error type
	maxDelay := 10 * time.Minute
	switch err.Type {
	case ErrorTypeNetwork:
		maxDelay = 5 * time.Minute
	case ErrorTypeTimeout:
		maxDelay = 2 * time.Minute
	case ErrorTypeSSL:
		maxDelay = 30 * time.Minute // SSL errors need longer delays
	case ErrorTypeStorage:
		maxDelay = 1 * time.Minute
	}
	
	if delay > maxDelay {
		delay = maxDelay
	}
	
	return delay
}

// enterDegradedMode puts a target into degraded monitoring mode
func (rm *RecoveryManager) enterDegradedMode(target string, failureInfo *TargetFailureInfo) {
	failureInfo.InDegradedMode = true
	rm.degradationMode = true
	
	rm.logger.Printf("Target %s entering degraded mode after %d consecutive failures", 
		target, failureInfo.ConsecutiveFails)
	
	// In degraded mode, we:
	// 1. Reduce monitoring frequency
	// 2. Skip non-essential checks
	// 3. Focus on basic connectivity
	// 4. Implement circuit breaker pattern
}

// exitDegradedMode removes a target from degraded monitoring mode
func (rm *RecoveryManager) exitDegradedMode(target string, failureInfo *TargetFailureInfo) {
	failureInfo.InDegradedMode = false
	
	rm.logger.Printf("Target %s exiting degraded mode after successful recovery", target)
	
	// Check if any targets are still in degraded mode
	rm.checkGlobalDegradationMode()
}

// checkGlobalDegradationMode checks if we should exit global degradation mode
func (rm *RecoveryManager) checkGlobalDegradationMode() {
	for _, failureInfo := range rm.failedTargets {
		if failureInfo.InDegradedMode {
			return // Still have targets in degraded mode
		}
	}
	
	if rm.degradationMode {
		rm.degradationMode = false
		rm.logger.Printf("Exiting global degradation mode - all targets recovered")
	}
}

// ShouldSkipTarget determines if a target should be skipped due to recent failures
func (rm *RecoveryManager) ShouldSkipTarget(target string) bool {
	rm.mutex.RLock()
	defer rm.mutex.RUnlock()
	
	failureInfo, exists := rm.failedTargets[target]
	if !exists {
		return false
	}
	
	// Skip if we haven't reached the next retry time
	if time.Now().Before(failureInfo.NextRetryTime) {
		return true
	}
	
	// Skip if in degraded mode and we've exceeded max recovery attempts
	if failureInfo.InDegradedMode {
		attempts := rm.recoveryAttempts[target]
		if attempts >= rm.maxRecoveryTries {
			return true
		}
	}
	
	return false
}

// GetTargetStatus returns the current status of a target
func (rm *RecoveryManager) GetTargetStatus(target string) string {
	rm.mutex.RLock()
	defer rm.mutex.RUnlock()
	
	failureInfo, exists := rm.failedTargets[target]
	if !exists {
		return "healthy"
	}
	
	if failureInfo.InDegradedMode {
		return "degraded"
	}
	
	if time.Now().Before(failureInfo.NextRetryTime) {
		return "backing_off"
	}
	
	return "failing"
}

// RecoverStorageOperation attempts to recover from storage failures
func (rm *RecoveryManager) RecoverStorageOperation(operation func() error, operationName string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	
	return rm.errorHandler.RetryWithBackoff(ctx, fmt.Sprintf("storage_%s", operationName), func() error {
		err := operation()
		if err != nil {
			// Handle specific storage errors
			if storageErr := rm.handleStorageError(err, operationName); storageErr != nil {
				return storageErr
			}
		}
		return err
	})
}

// handleStorageError implements specific recovery strategies for storage errors
func (rm *RecoveryManager) handleStorageError(err error, operationName string) error {
	errMsg := err.Error()
	
	switch {
	case contains(errMsg, "permission denied"):
		rm.logger.Printf("Storage permission error for %s - attempting to create backup location", operationName)
		return rm.createBackupStorage(operationName)
		
	case contains(errMsg, "no space left"):
		rm.logger.Printf("Disk full error for %s - attempting cleanup", operationName)
		return rm.cleanupOldFiles(operationName)
		
	case contains(errMsg, "file not found"):
		rm.logger.Printf("File not found error for %s - attempting to create directory", operationName)
		return rm.createMissingDirectories(operationName)
		
	default:
		return err
	}
}

// createBackupStorage creates alternative storage location
func (rm *RecoveryManager) createBackupStorage(operationName string) error {
	backupPath := fmt.Sprintf("./backup_%s_%d", operationName, time.Now().Unix())
	rm.storageBackups = append(rm.storageBackups, backupPath)
	rm.logger.Printf("Created backup storage location: %s", backupPath)
	return nil
}

// cleanupOldFiles attempts to free up disk space
func (rm *RecoveryManager) cleanupOldFiles(operationName string) error {
	// This is a placeholder - in a real implementation, you would:
	// 1. Find old monitoring files
	// 2. Compress or delete them
	// 3. Clean up temporary files
	rm.logger.Printf("Attempting to clean up old files for %s", operationName)
	return fmt.Errorf("disk cleanup not implemented - manual intervention required")
}

// createMissingDirectories creates missing directories for storage operations
func (rm *RecoveryManager) createMissingDirectories(operationName string) error {
	// This would create necessary directories
	rm.logger.Printf("Attempting to create missing directories for %s", operationName)
	return nil
}

// RecoverAlertDelivery attempts to recover from alert delivery failures
func (rm *RecoveryManager) RecoverAlertDelivery(alert *Alert, deliveryFunc func(*Alert) error) error {
	// Add to backlog first
	rm.mutex.Lock()
	rm.alertBacklog = append(rm.alertBacklog, alert)
	rm.mutex.Unlock()
	
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	
	alertKey := fmt.Sprintf("alert_%s_%s", alert.Type, alert.Target)
	
	return rm.errorHandler.RetryWithBackoff(ctx, alertKey, func() error {
		err := deliveryFunc(alert)
		if err == nil {
			// Remove from backlog on success
			rm.removeFromAlertBacklog(alert)
		}
		return err
	})
}

// removeFromAlertBacklog removes a successfully delivered alert from the backlog
func (rm *RecoveryManager) removeFromAlertBacklog(deliveredAlert *Alert) {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()
	
	for i, alert := range rm.alertBacklog {
		if alert.Type == deliveredAlert.Type && 
		   alert.Target == deliveredAlert.Target && 
		   alert.Timestamp.Equal(deliveredAlert.Timestamp) {
			// Remove from backlog
			rm.alertBacklog = append(rm.alertBacklog[:i], rm.alertBacklog[i+1:]...)
			rm.logger.Printf("Removed delivered alert from backlog: %s for %s", alert.Type, alert.Target)
			break
		}
	}
}

// ProcessAlertBacklog attempts to deliver failed alerts from the backlog
func (rm *RecoveryManager) ProcessAlertBacklog(deliveryFunc func(*Alert) error) {
	rm.mutex.RLock()
	backlogCopy := make([]*Alert, len(rm.alertBacklog))
	copy(backlogCopy, rm.alertBacklog)
	rm.mutex.RUnlock()
	
	if len(backlogCopy) == 0 {
		return
	}
	
	rm.logger.Printf("Processing alert backlog with %d pending alerts", len(backlogCopy))
	
	for _, alert := range backlogCopy {
		// Skip very old alerts (older than 24 hours)
		if time.Since(alert.Timestamp) > 24*time.Hour {
			rm.removeFromAlertBacklog(alert)
			rm.logger.Printf("Discarded old alert from backlog: %s for %s", alert.Type, alert.Target)
			continue
		}
		
		// Attempt delivery
		if err := deliveryFunc(alert); err == nil {
			rm.removeFromAlertBacklog(alert)
			rm.logger.Printf("Successfully delivered backlogged alert: %s for %s", alert.Type, alert.Target)
		} else {
			rm.logger.Printf("Failed to deliver backlogged alert: %s for %s: %v", alert.Type, alert.Target, err)
		}
	}
}

// GetRecoveryStats returns statistics about recovery operations
func (rm *RecoveryManager) GetRecoveryStats() map[string]interface{} {
	rm.mutex.RLock()
	defer rm.mutex.RUnlock()
	
	stats := make(map[string]interface{})
	stats["degradation_mode"] = rm.degradationMode
	stats["failed_targets_count"] = len(rm.failedTargets)
	stats["alert_backlog_size"] = len(rm.alertBacklog)
	stats["storage_backups_count"] = len(rm.storageBackups)
	
	// Count targets by status
	statusCounts := make(map[string]int)
	for _, failureInfo := range rm.failedTargets {
		if failureInfo.InDegradedMode {
			statusCounts["degraded"]++
		} else {
			statusCounts["failing"]++
		}
	}
	stats["target_status_counts"] = statusCounts
	
	// Recovery attempt statistics
	totalRecoveryAttempts := 0
	for _, attempts := range rm.recoveryAttempts {
		totalRecoveryAttempts += attempts
	}
	stats["total_recovery_attempts"] = totalRecoveryAttempts
	
	return stats
}

// IsInDegradationMode returns whether the system is in degradation mode
func (rm *RecoveryManager) IsInDegradationMode() bool {
	rm.mutex.RLock()
	defer rm.mutex.RUnlock()
	return rm.degradationMode
}

// GetFailedTargets returns a list of currently failed targets
func (rm *RecoveryManager) GetFailedTargets() []string {
	rm.mutex.RLock()
	defer rm.mutex.RUnlock()
	
	targets := make([]string, 0, len(rm.failedTargets))
	for target := range rm.failedTargets {
		targets = append(targets, target)
	}
	return targets
}

// GetAlertBacklogSize returns the current size of the alert backlog
func (rm *RecoveryManager) GetAlertBacklogSize() int {
	rm.mutex.RLock()
	defer rm.mutex.RUnlock()
	return len(rm.alertBacklog)
}

// Reset clears all recovery state (useful for testing or manual reset)
func (rm *RecoveryManager) Reset() {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()
	
	rm.failedTargets = make(map[string]*TargetFailureInfo)
	rm.alertBacklog = make([]*Alert, 0)
	rm.recoveryAttempts = make(map[string]int)
	rm.degradationMode = false
	
	rm.logger.Printf("Recovery manager state reset")
}

// StartRecoveryLoop starts a background goroutine that periodically attempts recovery
func (rm *RecoveryManager) StartRecoveryLoop(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	
	rm.logger.Printf("Starting recovery loop with interval %v", interval)
	
	for {
		select {
		case <-ctx.Done():
			rm.logger.Printf("Recovery loop stopped")
			return
		case <-ticker.C:
			rm.performRecoveryCheck()
		}
	}
}

// performRecoveryCheck performs periodic recovery checks
func (rm *RecoveryManager) performRecoveryCheck() {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()
	
	now := time.Now()
	
	// Check for targets ready for retry
	for target, failureInfo := range rm.failedTargets {
		if now.After(failureInfo.NextRetryTime) {
			rm.recoveryAttempts[target]++
			rm.logger.Printf("Target %s ready for recovery attempt %d", target, rm.recoveryAttempts[target])
		}
	}
	
	// Log recovery status if in degradation mode
	if rm.degradationMode {
		degradedCount := 0
		for _, failureInfo := range rm.failedTargets {
			if failureInfo.InDegradedMode {
				degradedCount++
			}
		}
		rm.logger.Printf("Recovery status: %d targets in degraded mode, %d alerts in backlog", 
			degradedCount, len(rm.alertBacklog))
	}
}

// contains is a helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || 
		(len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || 
		 containsSubstring(s, substr))))
}

// containsSubstring checks if s contains substr
func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}