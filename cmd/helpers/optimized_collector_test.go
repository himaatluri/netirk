package helpers

import (
	"log"
	"os"
	"testing"
	"time"
)

func TestOptimizedCollectMetrics(t *testing.T) {
	// Create performance manager
	perfConfig := DefaultPerformanceConfig()
	perfManager := NewPerformanceManager(perfConfig, nil)
	defer perfManager.Cleanup()
	
	// Create storage
	storage := NewInMemoryStorage(MonitoringConfig{})
	
	// Create test targets
	targets := []ParsedTargetConfig{
		{
			URL:            "https://httpbin.org/status/200",
			Timeout:        10 * time.Second,
			ExpectedStatus: 200,
			Headers:        make(map[string]string),
		},
		{
			URL:     "8.8.8.8:53",
			Timeout: 5 * time.Second,
			Headers: make(map[string]string),
		},
	}
	
	logger := log.New(os.Stdout, "TEST: ", log.LstdFlags)
	
	err := OptimizedCollectMetrics(targets, storage, perfManager, logger)
	if err != nil {
		t.Fatalf("OptimizedCollectMetrics failed: %v", err)
	}
	
	// Check that data was stored
	session := storage.GetSession()
	if len(session.Data) != 2 {
		t.Errorf("Expected 2 data points, got: %d", len(session.Data))
	}
	
	// Check performance metrics were recorded
	metrics := perfManager.GetMetrics()
	if metrics.TotalRequests != 2 {
		t.Errorf("Expected 2 requests recorded, got: %d", metrics.TotalRequests)
	}
}

func TestOptimizedCollectMetrics_EmptyTargets(t *testing.T) {
	perfManager := NewPerformanceManager(DefaultPerformanceConfig(), nil)
	defer perfManager.Cleanup()
	
	storage := NewInMemoryStorage(MonitoringConfig{})
	logger := log.New(os.Stdout, "TEST: ", log.LstdFlags)
	
	err := OptimizedCollectMetrics([]ParsedTargetConfig{}, storage, perfManager, logger)
	if err == nil {
		t.Error("Expected error for empty targets")
	}
	
	if err.Error() != "no targets provided for monitoring" {
		t.Errorf("Expected specific error message, got: %s", err.Error())
	}
}

func TestOptimizedCollectMetrics_NilParameters(t *testing.T) {
	targets := []ParsedTargetConfig{
		{
			URL:     "https://example.com",
			Timeout: 10 * time.Second,
			Headers: make(map[string]string),
		},
	}
	
	// Test with nil performance manager - should handle gracefully
	storage := NewInMemoryStorage(MonitoringConfig{})
	logger := log.New(os.Stdout, "TEST: ", log.LstdFlags)
	
	// This should not panic but may return an error
	err := OptimizedCollectMetrics(targets, storage, nil, logger)
	// We expect this to fail gracefully, not panic
	if err == nil {
		t.Error("Expected error for nil performance manager")
	}
	
	// Test with nil storage
	perfManager := NewPerformanceManager(DefaultPerformanceConfig(), nil)
	defer perfManager.Cleanup()
	
	err = OptimizedCollectMetrics(targets, nil, perfManager, logger)
	if err == nil {
		t.Error("Expected error for nil storage")
	}
	
	// Test with nil logger (should work with default logger)
	storage = NewInMemoryStorage(MonitoringConfig{})
	err = OptimizedCollectMetrics(targets, storage, perfManager, nil)
	// This should not error as it should use a default logger
	if err != nil && err.Error() != "no targets provided for monitoring" {
		// Only fail if it's not the expected error from actual collection
		t.Errorf("Unexpected error with nil logger: %v", err)
	}
}

func TestOptimizedCollectMetrics_ConcurrencyLimits(t *testing.T) {
	// Create performance manager with low concurrency limit
	perfConfig := PerformanceConfig{
		MaxConcurrentTargets: 1, // Very low limit
		ConnectionPoolSize:   10,
		MemoryLimitMB:       50,
		GCInterval:          1 * time.Minute,
		MetricsInterval:     30 * time.Second,
	}
	perfManager := NewPerformanceManager(perfConfig, nil)
	defer perfManager.Cleanup()
	
	storage := NewInMemoryStorage(MonitoringConfig{})
	logger := log.New(os.Stdout, "TEST: ", log.LstdFlags)
	
	// Create multiple targets to test concurrency limiting
	targets := []ParsedTargetConfig{
		{
			URL:     "https://httpbin.org/delay/1",
			Timeout: 5 * time.Second,
			Headers: make(map[string]string),
		},
		{
			URL:     "https://httpbin.org/delay/1",
			Timeout: 5 * time.Second,
			Headers: make(map[string]string),
		},
		{
			URL:     "https://httpbin.org/delay/1",
			Timeout: 5 * time.Second,
			Headers: make(map[string]string),
		},
	}
	
	start := time.Now()
	err := OptimizedCollectMetrics(targets, storage, perfManager, logger)
	duration := time.Since(start)
	
	if err != nil {
		t.Fatalf("OptimizedCollectMetrics failed: %v", err)
	}
	
	// With concurrency limit of 1, this should take longer than if all ran concurrently
	// Each request has a 1-second delay, so with limit of 1, it should take at least 3 seconds
	if duration < 2*time.Second {
		t.Errorf("Expected duration >= 2s with concurrency limit, got: %v", duration)
	}
	
	// Check that all targets were processed
	session := storage.GetSession()
	if len(session.Data) != 3 {
		t.Errorf("Expected 3 data points, got: %d", len(session.Data))
	}
}

func TestOptimizedCollectMetrics_ErrorHandling(t *testing.T) {
	perfManager := NewPerformanceManager(DefaultPerformanceConfig(), nil)
	defer perfManager.Cleanup()
	
	storage := NewInMemoryStorage(MonitoringConfig{})
	logger := log.New(os.Stdout, "TEST: ", log.LstdFlags)
	
	// Create targets with some that will fail
	targets := []ParsedTargetConfig{
		{
			URL:     "https://httpbin.org/status/200",
			Timeout: 10 * time.Second,
			Headers: make(map[string]string),
		},
		{
			URL:     "https://this-domain-does-not-exist-12345.com",
			Timeout: 5 * time.Second,
			Headers: make(map[string]string),
		},
		{
			URL:     "https://httpbin.org/status/404",
			Timeout: 10 * time.Second,
			ExpectedStatus: 200, // Will fail due to status mismatch
			Headers: make(map[string]string),
		},
	}
	
	err := OptimizedCollectMetrics(targets, storage, perfManager, logger)
	if err != nil {
		t.Fatalf("OptimizedCollectMetrics failed: %v", err)
	}
	
	// Check that all targets were processed (including failures)
	session := storage.GetSession()
	if len(session.Data) != 3 {
		t.Errorf("Expected 3 data points, got: %d", len(session.Data))
	}
	
	// Check that failures were recorded properly
	successCount := 0
	failureCount := 0
	for _, data := range session.Data {
		if data.Status == "success" {
			successCount++
		} else if data.Status == "failed" {
			failureCount++
		}
	}
	
	if successCount != 1 {
		t.Errorf("Expected 1 success, got: %d", successCount)
	}
	
	if failureCount != 2 {
		t.Errorf("Expected 2 failures, got: %d", failureCount)
	}
}

func TestOptimizedCollectMetrics_PerformanceTracking(t *testing.T) {
	perfManager := NewPerformanceManager(DefaultPerformanceConfig(), nil)
	defer perfManager.Cleanup()
	
	storage := NewInMemoryStorage(MonitoringConfig{})
	logger := log.New(os.Stdout, "TEST: ", log.LstdFlags)
	
	targets := []ParsedTargetConfig{
		{
			URL:     "https://httpbin.org/status/200",
			Timeout: 10 * time.Second,
			Headers: make(map[string]string),
		},
	}
	
	// Get initial metrics
	initialMetrics := perfManager.GetMetrics()
	
	err := OptimizedCollectMetrics(targets, storage, perfManager, logger)
	if err != nil {
		t.Fatalf("OptimizedCollectMetrics failed: %v", err)
	}
	
	// Get final metrics
	finalMetrics := perfManager.GetMetrics()
	
	// Check that metrics were updated
	if finalMetrics.TotalRequests <= initialMetrics.TotalRequests {
		t.Error("Expected total requests to increase")
	}
	
	if finalMetrics.AverageResponseTime <= 0 {
		t.Error("Expected positive average response time")
	}
}

func TestOptimizedCollectMetrics_ResourceUtilization(t *testing.T) {
	perfManager := NewPerformanceManager(DefaultPerformanceConfig(), nil)
	defer perfManager.Cleanup()
	
	storage := NewInMemoryStorage(MonitoringConfig{})
	logger := log.New(os.Stdout, "TEST: ", log.LstdFlags)
	
	targets := []ParsedTargetConfig{
		{
			URL:     "https://httpbin.org/status/200",
			Timeout: 10 * time.Second,
			Headers: make(map[string]string),
		},
	}
	
	err := OptimizedCollectMetrics(targets, storage, perfManager, logger)
	if err != nil {
		t.Fatalf("OptimizedCollectMetrics failed: %v", err)
	}
	
	// Check resource utilization
	utilization := perfManager.GetResourceUtilization()
	
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
	
	// Check that total requests was incremented
	if utilization["total_requests"].(int64) != 1 {
		t.Errorf("Expected total_requests to be 1, got: %v", utilization["total_requests"])
	}
}

func TestOptimizedCollectMetrics_MemoryManagement(t *testing.T) {
	// Create performance manager with very low memory limit for testing
	perfConfig := PerformanceConfig{
		MaxConcurrentTargets: 10,
		ConnectionPoolSize:   10,
		MemoryLimitMB:       1, // Very low limit
		GCInterval:          1 * time.Minute,
		MetricsInterval:     30 * time.Second,
	}
	perfManager := NewPerformanceManager(perfConfig, nil)
	defer perfManager.Cleanup()
	
	storage := NewInMemoryStorage(MonitoringConfig{})
	logger := log.New(os.Stdout, "TEST: ", log.LstdFlags)
	
	targets := []ParsedTargetConfig{
		{
			URL:     "https://httpbin.org/status/200",
			Timeout: 10 * time.Second,
			Headers: make(map[string]string),
		},
	}
	
	err := OptimizedCollectMetrics(targets, storage, perfManager, logger)
	if err != nil {
		t.Fatalf("OptimizedCollectMetrics failed: %v", err)
	}
	
	// Check memory usage
	withinLimit, memoryUsage := perfManager.CheckMemoryUsage()
	
	// Memory usage should be reported
	if memoryUsage <= 0 {
		t.Error("Expected positive memory usage")
	}
	
	// With a 1MB limit, we should likely exceed it, but the function should still complete
	t.Logf("Memory usage: %.2f MB, within limit: %v", memoryUsage, withinLimit)
}

func TestOptimizedCollectMetrics_HTTPClientOptimization(t *testing.T) {
	perfManager := NewPerformanceManager(DefaultPerformanceConfig(), nil)
	defer perfManager.Cleanup()
	
	// Get the optimized HTTP client
	client := perfManager.GetOptimizedHTTPClient()
	if client == nil {
		t.Fatal("Expected optimized HTTP client, got nil")
	}
	
	// Check that client has reasonable timeout
	if client.Timeout <= 0 {
		t.Error("Expected positive client timeout")
	}
	
	storage := NewInMemoryStorage(MonitoringConfig{})
	logger := log.New(os.Stdout, "TEST: ", log.LstdFlags)
	
	targets := []ParsedTargetConfig{
		{
			URL:     "https://httpbin.org/status/200",
			Timeout: 10 * time.Second,
			Headers: make(map[string]string),
		},
	}
	
	err := OptimizedCollectMetrics(targets, storage, perfManager, logger)
	if err != nil {
		t.Fatalf("OptimizedCollectMetrics failed: %v", err)
	}
	
	// Verify that the optimized client was used effectively
	session := storage.GetSession()
	if len(session.Data) != 1 {
		t.Errorf("Expected 1 data point, got: %d", len(session.Data))
	}
	
	if session.Data[0].Status != "success" {
		t.Errorf("Expected successful request, got status: %s", session.Data[0].Status)
	}
}

func TestOptimizedCollectMetrics_LongRunningOptimization(t *testing.T) {
	perfManager := NewPerformanceManager(DefaultPerformanceConfig(), nil)
	defer perfManager.Cleanup()
	
	// Optimize for long running
	perfManager.OptimizeForLongRunning()
	
	storage := NewInMemoryStorage(MonitoringConfig{})
	logger := log.New(os.Stdout, "TEST: ", log.LstdFlags)
	
	targets := []ParsedTargetConfig{
		{
			URL:     "https://httpbin.org/status/200",
			Timeout: 10 * time.Second,
			Headers: make(map[string]string),
		},
	}
	
	err := OptimizedCollectMetrics(targets, storage, perfManager, logger)
	if err != nil {
		t.Fatalf("OptimizedCollectMetrics failed with long-running optimization: %v", err)
	}
	
	// Check that data was collected successfully even with long-running optimizations
	session := storage.GetSession()
	if len(session.Data) != 1 {
		t.Errorf("Expected 1 data point, got: %d", len(session.Data))
	}
}

// Benchmark tests
func BenchmarkOptimizedCollectMetrics_SingleTarget(b *testing.B) {
	perfManager := NewPerformanceManager(DefaultPerformanceConfig(), nil)
	defer perfManager.Cleanup()
	
	storage := NewInMemoryStorage(MonitoringConfig{})
	logger := log.New(os.Stdout, "BENCH: ", log.LstdFlags)
	
	targets := []ParsedTargetConfig{
		{
			URL:     "https://httpbin.org/status/200",
			Timeout: 10 * time.Second,
			Headers: make(map[string]string),
		},
	}
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		storage.ClearData() // Clear previous data
		err := OptimizedCollectMetrics(targets, storage, perfManager, logger)
		if err != nil {
			b.Fatalf("OptimizedCollectMetrics failed: %v", err)
		}
	}
}

func BenchmarkOptimizedCollectMetrics_MultipleTargets(b *testing.B) {
	perfManager := NewPerformanceManager(DefaultPerformanceConfig(), nil)
	defer perfManager.Cleanup()
	
	storage := NewInMemoryStorage(MonitoringConfig{})
	logger := log.New(os.Stdout, "BENCH: ", log.LstdFlags)
	
	targets := []ParsedTargetConfig{
		{
			URL:     "https://httpbin.org/status/200",
			Timeout: 10 * time.Second,
			Headers: make(map[string]string),
		},
		{
			URL:     "https://httpbin.org/status/201",
			Timeout: 10 * time.Second,
			Headers: make(map[string]string),
		},
		{
			URL:     "https://httpbin.org/status/202",
			Timeout: 10 * time.Second,
			Headers: make(map[string]string),
		},
	}
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		storage.ClearData() // Clear previous data
		err := OptimizedCollectMetrics(targets, storage, perfManager, logger)
		if err != nil {
			b.Fatalf("OptimizedCollectMetrics failed: %v", err)
		}
	}
}