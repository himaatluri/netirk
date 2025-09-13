package helpers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"testing"
	"time"
)

func TestDefaultBenchmarkConfig(t *testing.T) {
	config := DefaultBenchmarkConfig()

	// Test that all required fields are set
	if len(config.TargetCounts) == 0 {
		t.Error("TargetCounts should not be empty")
	}

	if len(config.ConcurrencyLevels) == 0 {
		t.Error("ConcurrencyLevels should not be empty")
	}

	if config.Duration <= 0 {
		t.Error("Duration should be positive")
	}

	if config.WarmupDuration <= 0 {
		t.Error("WarmupDuration should be positive")
	}

	if config.Iterations <= 0 {
		t.Error("Iterations should be positive")
	}

	// Test reasonable defaults
	if config.Duration < 10*time.Second {
		t.Error("Duration should be at least 10 seconds for meaningful benchmarks")
	}

	if config.Iterations < 1 {
		t.Error("Iterations should be at least 1")
	}
}

func TestNewBenchmarkSuite(t *testing.T) {
	config := DefaultBenchmarkConfig()
	perfManager := NewPerformanceManager(DefaultPerformanceConfig(), nil)
	defer perfManager.Cleanup()

	logger := log.New(os.Stdout, "TEST: ", log.LstdFlags)

	// Test with valid parameters
	suite := NewBenchmarkSuite(config, perfManager, logger)
	if suite == nil {
		t.Fatal("NewBenchmarkSuite returned nil")
	}

	if suite.config.Duration != config.Duration {
		t.Error("Config not properly set")
	}

	if suite.performanceManager != perfManager {
		t.Error("Performance manager not properly set")
	}

	if suite.logger != logger {
		t.Error("Logger not properly set")
	}

	// Test with nil logger (should use default)
	suite2 := NewBenchmarkSuite(config, perfManager, nil)
	if suite2 == nil {
		t.Fatal("NewBenchmarkSuite with nil logger returned nil")
	}

	if suite2.logger == nil {
		t.Error("Default logger should be set when nil is passed")
	}
}

func TestBenchmarkSuite_CreateTestTargets(t *testing.T) {
	config := DefaultBenchmarkConfig()
	perfManager := NewPerformanceManager(DefaultPerformanceConfig(), nil)
	defer perfManager.Cleanup()

	suite := NewBenchmarkSuite(config, perfManager, nil)

	// Test creating different numbers of targets
	testCases := []int{1, 5, 10, 25}

	for _, count := range testCases {
		t.Run(fmt.Sprintf("count_%d", count), func(t *testing.T) {
			targets := suite.createTestTargets(count)

			if len(targets) != count {
				t.Errorf("Expected %d targets, got %d", count, len(targets))
			}

			// Verify all targets have required fields
			for i, target := range targets {
				if target.URL == "" {
					t.Errorf("Target %d has empty URL", i)
				}

				if target.Timeout <= 0 {
					t.Errorf("Target %d has invalid timeout: %v", i, target.Timeout)
				}

				// Check that we have a mix of HTTP and TCP targets
				isHTTP := strings.HasPrefix(target.URL, "http")
				isTCP := strings.Contains(target.URL, ":") && !isHTTP

				if !isHTTP && !isTCP {
					t.Errorf("Target %d has invalid URL format: %s", i, target.URL)
				}

				// HTTP targets should have expected status
				if isHTTP && target.ExpectedStatus == 0 {
					t.Errorf("HTTP target %d should have ExpectedStatus set", i)
				}
			}
		})
	}
}

func TestBenchmarkSuite_CalculateLatencyStats(t *testing.T) {
	config := DefaultBenchmarkConfig()
	perfManager := NewPerformanceManager(DefaultPerformanceConfig(), nil)
	defer perfManager.Cleanup()

	suite := NewBenchmarkSuite(config, perfManager, nil)

	// Test with empty latencies
	avg, min, max, p95, p99 := suite.calculateLatencyStats([]time.Duration{})
	if avg != 0 || min != 0 || max != 0 || p95 != 0 || p99 != 0 {
		t.Error("Empty latencies should return all zeros")
	}

	// Test with single latency
	singleLatency := []time.Duration{100 * time.Millisecond}
	avg, min, max, p95, p99 = suite.calculateLatencyStats(singleLatency)
	expected := 100 * time.Millisecond

	if avg != expected || min != expected || max != expected || p95 != expected || p99 != expected {
		t.Error("Single latency should return same value for all stats")
	}

	// Test with multiple latencies
	latencies := []time.Duration{
		50 * time.Millisecond,
		100 * time.Millisecond,
		150 * time.Millisecond,
		200 * time.Millisecond,
		250 * time.Millisecond,
	}

	avg, min, max, p95, p99 = suite.calculateLatencyStats(latencies)

	if min != 50*time.Millisecond {
		t.Errorf("Expected min 50ms, got %v", min)
	}

	if max != 250*time.Millisecond {
		t.Errorf("Expected max 250ms, got %v", max)
	}

	if avg != 150*time.Millisecond {
		t.Errorf("Expected avg 150ms, got %v", avg)
	}

	// P95 and P99 should be reasonable
	if p95 < 200*time.Millisecond || p95 > 250*time.Millisecond {
		t.Errorf("P95 should be between 200-250ms, got %v", p95)
	}

	if p99 < 200*time.Millisecond || p99 > 250*time.Millisecond {
		t.Errorf("P99 should be between 200-250ms, got %v", p99)
	}
}

func TestBenchmarkSuite_GetResults(t *testing.T) {
	config := DefaultBenchmarkConfig()
	perfManager := NewPerformanceManager(DefaultPerformanceConfig(), nil)
	defer perfManager.Cleanup()

	suite := NewBenchmarkSuite(config, perfManager, nil)

	// Initially should be empty
	results := suite.GetResults()
	if len(results) != 0 {
		t.Error("Initial results should be empty")
	}

	// Add a test result
	testResult := BenchmarkResult{
		TargetCount:       5,
		ConcurrencyLevel:  2,
		Duration:          30 * time.Second,
		TotalRequests:     100,
		RequestsPerSecond: 3.33,
		AverageLatency:    300 * time.Millisecond,
	}

	suite.mutex.Lock()
	suite.results = append(suite.results, testResult)
	suite.mutex.Unlock()

	// Should return the result
	results = suite.GetResults()
	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}

	if results[0].TargetCount != testResult.TargetCount {
		t.Error("Result not properly returned")
	}

	// Test that returned slice is a copy (modifying it shouldn't affect original)
	results[0].TargetCount = 999
	originalResults := suite.GetResults()
	if originalResults[0].TargetCount == 999 {
		t.Error("GetResults should return a copy, not the original slice")
	}
}

func TestBenchmarkSuite_GenerateReport(t *testing.T) {
	config := DefaultBenchmarkConfig()
	perfManager := NewPerformanceManager(DefaultPerformanceConfig(), nil)
	defer perfManager.Cleanup()

	suite := NewBenchmarkSuite(config, perfManager, nil)

	// Test with no results
	report := suite.GenerateReport()
	if !strings.Contains(report, "No benchmark results available") {
		t.Error("Report should indicate no results available")
	}

	// Add test results
	testResults := []BenchmarkResult{
		{
			TargetCount:       5,
			ConcurrencyLevel:  2,
			Duration:          30 * time.Second,
			TotalRequests:     100,
			RequestsPerSecond: 3.33,
			AverageLatency:    300 * time.Millisecond,
			P95Latency:        400 * time.Millisecond,
			P99Latency:        500 * time.Millisecond,
			ErrorRate:         0.0,
			MemoryUsageMB:     10.5,
		},
		{
			TargetCount:       10,
			ConcurrencyLevel:  5,
			Duration:          30 * time.Second,
			TotalRequests:     200,
			RequestsPerSecond: 6.67,
			AverageLatency:    250 * time.Millisecond,
			P95Latency:        350 * time.Millisecond,
			P99Latency:        450 * time.Millisecond,
			ErrorRate:         1.0,
			MemoryUsageMB:     15.2,
		},
	}

	suite.mutex.Lock()
	suite.results = testResults
	suite.mutex.Unlock()

	report = suite.GenerateReport()

	// Test that report contains expected sections
	expectedSections := []string{
		"Performance Benchmark Report",
		"Total tests run:",
		"Test duration:",
		"Targets",
		"Concurrency",
		"Req/Sec",
		"Summary Statistics:",
		"Average requests/sec:",
		"Max requests/sec:",
		"Min requests/sec:",
		"Average memory usage:",
	}

	for _, section := range expectedSections {
		if !strings.Contains(report, section) {
			t.Errorf("Report should contain section: %s", section)
		}
	}

	// Test that report contains data from results
	if !strings.Contains(report, "3.33") {
		t.Error("Report should contain first result's req/sec")
	}

	if !strings.Contains(report, "6.67") {
		t.Error("Report should contain second result's req/sec")
	}

	// Test summary calculations
	if !strings.Contains(report, "5.00") { // Average req/sec: (3.33 + 6.67) / 2
		t.Error("Report should contain correct average req/sec")
	}
}

func TestBenchmarkResult_Structure(t *testing.T) {
	// Test that BenchmarkResult has all expected fields
	result := BenchmarkResult{
		TargetCount:       10,
		ConcurrencyLevel:  5,
		Duration:          30 * time.Second,
		TotalRequests:     150,
		RequestsPerSecond: 5.0,
		AverageLatency:    200 * time.Millisecond,
		MinLatency:        100 * time.Millisecond,
		MaxLatency:        500 * time.Millisecond,
		P95Latency:        400 * time.Millisecond,
		P99Latency:        450 * time.Millisecond,
		ErrorRate:         2.5,
		MemoryUsageMB:     12.8,
		GoroutineCount:    25,
		CPUUsagePercent:   15.5,
	}

	// Test that all fields are accessible and have expected values
	if result.TargetCount != 10 {
		t.Error("TargetCount field not accessible")
	}

	if result.RequestsPerSecond != 5.0 {
		t.Error("RequestsPerSecond field not accessible")
	}

	if result.AverageLatency != 200*time.Millisecond {
		t.Error("AverageLatency field not accessible")
	}

	if result.ErrorRate != 2.5 {
		t.Error("ErrorRate field not accessible")
	}

	// Test JSON tags (if any) by marshaling
	jsonData, err := json.Marshal(result)
	if err != nil {
		t.Errorf("Failed to marshal BenchmarkResult to JSON: %v", err)
	}

	if len(jsonData) == 0 {
		t.Error("JSON marshaling produced empty result")
	}

	// Test unmarshaling
	var unmarshaled BenchmarkResult
	err = json.Unmarshal(jsonData, &unmarshaled)
	if err != nil {
		t.Errorf("Failed to unmarshal BenchmarkResult from JSON: %v", err)
	}

	if unmarshaled.TargetCount != result.TargetCount {
		t.Error("JSON round-trip failed for TargetCount")
	}
}

// Mock test for runBenchmarkLoad (simplified version)
func TestBenchmarkSuite_RunBenchmarkLoad_Mock(t *testing.T) {
	config := BenchmarkConfig{
		Duration:       1 * time.Second, // Short duration for testing
		WarmupDuration: 100 * time.Millisecond,
	}

	perfManager := NewPerformanceManager(DefaultPerformanceConfig(), nil)
	defer perfManager.Cleanup()

	suite := NewBenchmarkSuite(config, perfManager, nil)

	// Create minimal test targets
	targets := []ParsedTargetConfig{
		{
			URL:     "http://httpbin.org/status/200",
			Timeout: 5 * time.Second,
		},
	}

	// Create test collector and storage
	collector := NewOptimizedCollector(perfManager, 5*time.Second, nil)
	storage := NewInMemoryStorage(MonitoringConfig{})

	// Test with short context
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	latencies := suite.runBenchmarkLoad(ctx, targets, collector, storage, 1)

	// Should have some latencies recorded
	if len(latencies) == 0 {
		t.Error("Expected some latencies to be recorded")
	}

	// All latencies should be positive
	for i, latency := range latencies {
		if latency <= 0 {
			t.Errorf("Latency %d should be positive, got %v", i, latency)
		}
	}

	// Should have some data in storage
	session := storage.GetSession()
	if len(session.Data) == 0 {
		t.Error("Expected some data to be stored")
	}
}

func BenchmarkBenchmarkSuite_CalculateLatencyStats(b *testing.B) {
	config := DefaultBenchmarkConfig()
	perfManager := NewPerformanceManager(DefaultPerformanceConfig(), nil)
	defer perfManager.Cleanup()

	suite := NewBenchmarkSuite(config, perfManager, nil)

	// Create test latencies
	latencies := make([]time.Duration, 1000)
	for i := range latencies {
		latencies[i] = time.Duration(i+1) * time.Millisecond
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		suite.calculateLatencyStats(latencies)
	}
}

func BenchmarkBenchmarkSuite_GenerateReport(b *testing.B) {
	config := DefaultBenchmarkConfig()
	perfManager := NewPerformanceManager(DefaultPerformanceConfig(), nil)
	defer perfManager.Cleanup()

	suite := NewBenchmarkSuite(config, perfManager, nil)

	// Add test results
	for i := 0; i < 100; i++ {
		result := BenchmarkResult{
			TargetCount:       i + 1,
			ConcurrencyLevel:  (i % 10) + 1,
			RequestsPerSecond: float64(i + 1),
			AverageLatency:    time.Duration(i+1) * time.Millisecond,
		}
		suite.results = append(suite.results, result)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		suite.GenerateReport()
	}
}