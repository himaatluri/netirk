package helpers

import (
	"log"
	"testing"
	"time"
)

// TestPerformanceOptimizationIntegration tests the complete performance optimization workflow
func TestPerformanceOptimizationIntegration(t *testing.T) {
	// Skip this test in short mode as it takes time
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create performance manager
	perfConfig := DefaultPerformanceConfig()
	perfConfig.MaxConcurrentTargets = 5 // Limit for testing
	perfConfig.MemoryLimitMB = 50        // Low limit for testing
	
	perfManager := NewPerformanceManager(perfConfig, nil)
	defer perfManager.Cleanup()

	// Create optimized storage
	monitoringConfig := MonitoringConfig{
		Targets:  []string{"https://httpbin.org/status/200", "https://httpbin.org/get"},
		Interval: 5 * time.Second,
	}
	
	storage := NewOptimizedStorage(monitoringConfig, perfManager, nil)

	// Create test targets
	targets := []ParsedTargetConfig{
		{
			URL:            "https://httpbin.org/status/200",
			Timeout:        10 * time.Second,
			ExpectedStatus: 200,
		},
		{
			URL:            "https://httpbin.org/get",
			Timeout:        10 * time.Second,
			ExpectedStatus: 200,
		},
	}

	// Run multiple collection cycles to test performance optimization
	for cycle := 0; cycle < 3; cycle++ {
		t.Logf("Running collection cycle %d", cycle+1)
		
		err := OptimizedCollectMetrics(targets, storage, perfManager, nil)
		if err != nil {
			t.Logf("Collection cycle %d completed with errors (may be expected): %v", cycle+1, err)
		}

		// Check performance metrics
		metrics := perfManager.GetMetrics()
		t.Logf("Cycle %d metrics: %d requests, avg response time: %v", 
			cycle+1, metrics.TotalRequests, metrics.AverageResponseTime)

		// Check memory usage
		withinLimit, memoryUsage := perfManager.CheckMemoryUsage()
		t.Logf("Cycle %d memory usage: %.2f MB, within limit: %v", 
			cycle+1, memoryUsage, withinLimit)

		// Brief pause between cycles
		time.Sleep(1 * time.Second)
	}

	// Check final storage stats
	stats := storage.GetStorageStats()
	t.Logf("Final storage stats: %v", stats)

	// Check final performance utilization
	utilization := perfManager.GetResourceUtilization()
	t.Logf("Final resource utilization: %v", utilization)

	// Verify that data was collected
	session := storage.GetSession()
	if len(session.Data) == 0 {
		t.Error("No data was collected during integration test")
	} else {
		t.Logf("Successfully collected %d data points", len(session.Data))
	}
}

// TestLongRunningOptimization tests optimizations for long-running sessions
func TestLongRunningOptimization(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping long-running test in short mode")
	}

	perfManager := NewPerformanceManager(DefaultPerformanceConfig(), nil)
	defer perfManager.Cleanup()

	// Get initial transport settings
	initialMaxIdle := perfManager.transport.MaxIdleConns
	initialIdleTimeout := perfManager.transport.IdleConnTimeout

	// Optimize for long running
	perfManager.OptimizeForLongRunning()

	// Verify optimizations were applied
	if perfManager.transport.MaxIdleConns >= initialMaxIdle {
		t.Error("MaxIdleConns should be reduced for long-running sessions")
	}

	if perfManager.transport.IdleConnTimeout >= initialIdleTimeout {
		t.Error("IdleConnTimeout should be reduced for long-running sessions")
	}

	t.Logf("Long-running optimizations applied: MaxIdleConns %d->%d, IdleTimeout %v->%v",
		initialMaxIdle, perfManager.transport.MaxIdleConns,
		initialIdleTimeout, perfManager.transport.IdleConnTimeout)
}

// TestMemoryManagement tests memory management features
func TestMemoryManagement(t *testing.T) {
	// Create performance manager with very low memory limit
	perfConfig := PerformanceConfig{
		MaxConcurrentTargets: 10,
		ConnectionPoolSize:   10,
		MemoryLimitMB:       1, // Very low for testing
		GCInterval:          1 * time.Second,
		MetricsInterval:     1 * time.Second,
	}

	perfManager := NewPerformanceManager(perfConfig, nil)
	defer perfManager.Cleanup()

	// Check initial memory usage
	withinLimit, initialMemory := perfManager.CheckMemoryUsage()
	t.Logf("Initial memory usage: %.2f MB, within limit: %v", initialMemory, withinLimit)

	// Force garbage collection
	perfManager.ForceGarbageCollection()

	// Check memory usage after GC
	withinLimitAfterGC, memoryAfterGC := perfManager.CheckMemoryUsage()
	t.Logf("Memory usage after GC: %.2f MB, within limit: %v", memoryAfterGC, withinLimitAfterGC)

	// Memory usage should be reported (may or may not be within the very low limit)
	if memoryAfterGC <= 0 {
		t.Error("Memory usage should be positive")
	}
}

// TestConcurrencyControl tests the concurrency control features
func TestConcurrencyControl(t *testing.T) {
	// Create performance manager with low concurrency limit
	perfConfig := PerformanceConfig{
		MaxConcurrentTargets: 2,
		ConnectionPoolSize:   10,
		MemoryLimitMB:       50,
		GCInterval:          1 * time.Minute,
		MetricsInterval:     30 * time.Second,
	}

	perfManager := NewPerformanceManager(perfConfig, nil)
	defer perfManager.Cleanup()

	// Create optimized collector
	_ = NewOptimizedCollector(perfManager, 5*time.Second, nil)

	// Create multiple targets to test concurrency control
	targets := []ParsedTargetConfig{
		{URL: "https://httpbin.org/delay/1", Timeout: 3 * time.Second},
		{URL: "https://httpbin.org/delay/1", Timeout: 3 * time.Second},
		{URL: "https://httpbin.org/delay/1", Timeout: 3 * time.Second},
	}

	storage := NewInMemoryStorage(MonitoringConfig{})

	// Run collection with concurrency control
	startTime := time.Now()
	err := OptimizedCollectMetrics(targets, storage, perfManager, log.Default())
	duration := time.Since(startTime)

	if err != nil {
		t.Logf("Collection completed with errors (may be expected): %v", err)
	}

	t.Logf("Concurrent collection took: %v", duration)

	// Check that some data was collected
	session := storage.GetSession()
	t.Logf("Collected %d data points with concurrency control", len(session.Data))

	// Check performance metrics
	metrics := perfManager.GetMetrics()
	t.Logf("Performance metrics: %d requests, avg response time: %v",
		metrics.TotalRequests, metrics.AverageResponseTime)
}

// BenchmarkPerformanceOptimizedCollection benchmarks the optimized collection
func BenchmarkPerformanceOptimizedCollection(b *testing.B) {
	perfManager := NewPerformanceManager(DefaultPerformanceConfig(), nil)
	defer perfManager.Cleanup()

	targets := []ParsedTargetConfig{
		{
			URL:            "https://httpbin.org/status/200",
			Timeout:        5 * time.Second,
			ExpectedStatus: 200,
		},
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		storage := NewInMemoryStorage(MonitoringConfig{})
		err := OptimizedCollectMetrics(targets, storage, perfManager, nil)
		if err != nil {
			b.Logf("Collection error: %v", err)
		}
	}
}

// BenchmarkMemoryOptimizedStorage benchmarks the optimized storage
func BenchmarkMemoryOptimizedStorage(b *testing.B) {
	perfManager := NewPerformanceManager(DefaultPerformanceConfig(), nil)
	defer perfManager.Cleanup()

	config := MonitoringConfig{
		Targets:  []string{"https://example.com"},
		Interval: 30 * time.Second,
	}

	storage := NewOptimizedStorage(config, perfManager, nil)

	data := &MonitoringData{
		Timestamp:    time.Now(),
		Target:       "https://example.com",
		Status:       "success",
		ResponseTime: 100 * time.Millisecond,
		StatusCode:   200,
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		err := storage.Store(data)
		if err != nil {
			b.Errorf("Failed to store data: %v", err)
		}
	}
}