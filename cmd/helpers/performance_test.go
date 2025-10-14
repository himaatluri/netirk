package helpers

import (
	"context"
	"log"
	"os"
	"testing"
	"time"
)

func TestNewPerformanceManager(t *testing.T) {
	config := DefaultPerformanceConfig()
	logger := log.New(os.Stdout, "TEST: ", log.LstdFlags)
	
	pm := NewPerformanceManager(config, logger)
	if pm == nil {
		t.Fatal("NewPerformanceManager returned nil")
	}
	
	defer pm.Cleanup()
	
	// Test that HTTP client is created
	client := pm.GetOptimizedHTTPClient()
	if client == nil {
		t.Error("GetOptimizedHTTPClient returned nil")
	}
	
	// Test that transport is configured
	if pm.transport == nil {
		t.Error("HTTP transport not configured")
	}
	
	// Test configuration values
	if pm.config.MaxConcurrentTargets != config.MaxConcurrentTargets {
		t.Errorf("Expected MaxConcurrentTargets %d, got %d", 
			config.MaxConcurrentTargets, pm.config.MaxConcurrentTargets)
	}
}

func TestDefaultPerformanceConfig(t *testing.T) {
	config := DefaultPerformanceConfig()
	
	if config.MaxConcurrentTargets <= 0 {
		t.Error("MaxConcurrentTargets should be positive")
	}
	
	if config.ConnectionPoolSize <= 0 {
		t.Error("ConnectionPoolSize should be positive")
	}
	
	if config.MemoryLimitMB <= 0 {
		t.Error("MemoryLimitMB should be positive")
	}
	
	if config.GCInterval <= 0 {
		t.Error("GCInterval should be positive")
	}
	
	if config.MetricsInterval <= 0 {
		t.Error("MetricsInterval should be positive")
	}
}

func TestPerformanceManager_AcquireReleaseSlot(t *testing.T) {
	config := PerformanceConfig{
		MaxConcurrentTargets: 2,
		ConnectionPoolSize:   10,
		MemoryLimitMB:       50,
		GCInterval:          1 * time.Minute,
		MetricsInterval:     30 * time.Second,
	}
	
	pm := NewPerformanceManager(config, nil)
	defer pm.Cleanup()
	
	ctx := context.Background()
	
	// Test acquiring slots
	err := pm.AcquireSlot(ctx)
	if err != nil {
		t.Errorf("Failed to acquire first slot: %v", err)
	}
	
	err = pm.AcquireSlot(ctx)
	if err != nil {
		t.Errorf("Failed to acquire second slot: %v", err)
	}
	
	// Test that third slot blocks (with timeout)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	
	err = pm.AcquireSlot(ctx)
	if err == nil {
		t.Error("Expected timeout error when acquiring third slot")
	}
	
	// Release slots
	pm.ReleaseSlot()
	pm.ReleaseSlot()
	
	// Should be able to acquire again
	ctx = context.Background()
	err = pm.AcquireSlot(ctx)
	if err != nil {
		t.Errorf("Failed to acquire slot after release: %v", err)
	}
	
	pm.ReleaseSlot()
}

func TestPerformanceManager_RecordRequest(t *testing.T) {
	config := DefaultPerformanceConfig()
	pm := NewPerformanceManager(config, nil)
	defer pm.Cleanup()
	
	// Record some requests
	pm.RecordRequest(100 * time.Millisecond)
	pm.RecordRequest(200 * time.Millisecond)
	pm.RecordRequest(150 * time.Millisecond)
	
	metrics := pm.GetMetrics()
	
	if metrics.TotalRequests != 3 {
		t.Errorf("Expected 3 total requests, got %d", metrics.TotalRequests)
	}
	
	if metrics.AverageResponseTime <= 0 {
		t.Error("Average response time should be positive")
	}
}

func TestPerformanceManager_CheckMemoryUsage(t *testing.T) {
	config := PerformanceConfig{
		MaxConcurrentTargets: 10,
		ConnectionPoolSize:   10,
		MemoryLimitMB:       1, // Very low limit for testing
		GCInterval:          1 * time.Minute,
		MetricsInterval:     30 * time.Second,
	}
	
	pm := NewPerformanceManager(config, nil)
	defer pm.Cleanup()
	
	withinLimit, memoryUsage := pm.CheckMemoryUsage()
	
	// Memory usage should be reported
	if memoryUsage <= 0 {
		t.Error("Memory usage should be positive")
	}
	
	// With a 1MB limit, we should likely exceed it
	t.Logf("Memory usage: %.2f MB, within limit: %v", memoryUsage, withinLimit)
}

func TestPerformanceManager_GetResourceUtilization(t *testing.T) {
	config := DefaultPerformanceConfig()
	pm := NewPerformanceManager(config, nil)
	defer pm.Cleanup()
	
	// Record some activity
	pm.RecordRequest(100 * time.Millisecond)
	
	utilization := pm.GetResourceUtilization()
	
	// Check that all expected keys are present
	expectedKeys := []string{
		"memory_usage_mb",
		"memory_limit_mb",
		"memory_utilization",
		"goroutine_count",
		"concurrent_requests",
		"max_concurrent",
		"total_requests",
		"avg_response_time",
		"last_gc_time",
	}
	
	for _, key := range expectedKeys {
		if _, exists := utilization[key]; !exists {
			t.Errorf("Expected key %s not found in resource utilization", key)
		}
	}
	
	// Check some specific values
	if utilization["memory_limit_mb"] != config.MemoryLimitMB {
		t.Errorf("Expected memory_limit_mb %d, got %v", 
			config.MemoryLimitMB, utilization["memory_limit_mb"])
	}
	
	if utilization["max_concurrent"] != config.MaxConcurrentTargets {
		t.Errorf("Expected max_concurrent %d, got %v", 
			config.MaxConcurrentTargets, utilization["max_concurrent"])
	}
	
	if utilization["total_requests"] != int64(1) {
		t.Errorf("Expected total_requests 1, got %v", utilization["total_requests"])
	}
}

func TestPerformanceManager_OptimizeForLongRunning(t *testing.T) {
	config := DefaultPerformanceConfig()
	pm := NewPerformanceManager(config, nil)
	defer pm.Cleanup()
	
	// Get initial transport settings
	initialMaxIdle := pm.transport.MaxIdleConns
	initialMaxIdlePerHost := pm.transport.MaxIdleConnsPerHost
	initialIdleTimeout := pm.transport.IdleConnTimeout
	
	// Optimize for long running
	pm.OptimizeForLongRunning()
	
	// Check that settings were changed
	if pm.transport.MaxIdleConns >= initialMaxIdle {
		t.Error("MaxIdleConns should be reduced for long-running sessions")
	}
	
	if pm.transport.MaxIdleConnsPerHost >= initialMaxIdlePerHost {
		t.Error("MaxIdleConnsPerHost should be reduced for long-running sessions")
	}
	
	if pm.transport.IdleConnTimeout >= initialIdleTimeout {
		t.Error("IdleConnTimeout should be reduced for long-running sessions")
	}
}

func TestPerformanceManager_Cleanup(t *testing.T) {
	config := DefaultPerformanceConfig()
	pm := NewPerformanceManager(config, nil)
	
	// Cleanup should not panic
	pm.Cleanup()
	
	// Multiple cleanups should not panic
	pm.Cleanup()
}

func TestPerformanceMetrics_ThreadSafety(t *testing.T) {
	config := DefaultPerformanceConfig()
	pm := NewPerformanceManager(config, nil)
	defer pm.Cleanup()
	
	// Test concurrent access to metrics
	done := make(chan bool)
	
	// Writer goroutine
	go func() {
		for i := 0; i < 100; i++ {
			pm.RecordRequest(time.Duration(i) * time.Millisecond)
		}
		done <- true
	}()
	
	// Reader goroutine
	go func() {
		for i := 0; i < 100; i++ {
			_ = pm.GetMetrics()
		}
		done <- true
	}()
	
	// Wait for both goroutines
	<-done
	<-done
	
	// Final check
	metrics := pm.GetMetrics()
	if metrics.TotalRequests != 100 {
		t.Errorf("Expected 100 total requests, got %d", metrics.TotalRequests)
	}
}

func BenchmarkPerformanceManager_RecordRequest(b *testing.B) {
	config := DefaultPerformanceConfig()
	pm := NewPerformanceManager(config, nil)
	defer pm.Cleanup()
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		pm.RecordRequest(time.Duration(i%1000) * time.Microsecond)
	}
}

func BenchmarkPerformanceManager_GetMetrics(b *testing.B) {
	config := DefaultPerformanceConfig()
	pm := NewPerformanceManager(config, nil)
	defer pm.Cleanup()
	
	// Add some data
	for i := 0; i < 1000; i++ {
		pm.RecordRequest(time.Duration(i) * time.Microsecond)
	}
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		_ = pm.GetMetrics()
	}
}