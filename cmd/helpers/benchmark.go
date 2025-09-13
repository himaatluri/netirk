package helpers

import (
	"context"
	"fmt"
	"log"
	"runtime"
	"sync"
	"time"
)

// BenchmarkConfig holds configuration for performance benchmarks
type BenchmarkConfig struct {
	TargetCounts      []int         // Number of targets to test with
	ConcurrencyLevels []int         // Concurrency levels to test
	Duration          time.Duration // Duration for each benchmark run
	WarmupDuration    time.Duration // Warmup duration before measurement
	Iterations        int           // Number of iterations per test
}

// BenchmarkResult holds the results of a performance benchmark
type BenchmarkResult struct {
	TargetCount       int           `json:"target_count"`
	ConcurrencyLevel  int           `json:"concurrency_level"`
	Duration          time.Duration `json:"duration"`
	TotalRequests     int64         `json:"total_requests"`
	RequestsPerSecond float64       `json:"requests_per_second"`
	AverageLatency    time.Duration `json:"average_latency"`
	MinLatency        time.Duration `json:"min_latency"`
	MaxLatency        time.Duration `json:"max_latency"`
	P95Latency        time.Duration `json:"p95_latency"`
	P99Latency        time.Duration `json:"p99_latency"`
	ErrorRate         float64       `json:"error_rate"`
	MemoryUsageMB     float64       `json:"memory_usage_mb"`
	GoroutineCount    int           `json:"goroutine_count"`
	CPUUsagePercent   float64       `json:"cpu_usage_percent"`
}

// BenchmarkSuite manages performance benchmarking for monitoring operations
type BenchmarkSuite struct {
	config          BenchmarkConfig
	performanceManager *PerformanceManager
	logger          *log.Logger
	results         []BenchmarkResult
	mutex           sync.RWMutex
}

// NewBenchmarkSuite creates a new benchmark suite
func NewBenchmarkSuite(config BenchmarkConfig, perfManager *PerformanceManager, logger *log.Logger) *BenchmarkSuite {
	if logger == nil {
		logger = log.Default()
	}

	return &BenchmarkSuite{
		config:          config,
		performanceManager: perfManager,
		logger:          logger,
		results:         make([]BenchmarkResult, 0),
	}
}

// DefaultBenchmarkConfig returns sensible defaults for benchmarking
func DefaultBenchmarkConfig() BenchmarkConfig {
	return BenchmarkConfig{
		TargetCounts:      []int{1, 5, 10, 25, 50, 100},
		ConcurrencyLevels: []int{1, 5, 10, 20, 50},
		Duration:          30 * time.Second,
		WarmupDuration:    5 * time.Second,
		Iterations:        3,
	}
}

// RunBenchmarks executes the complete benchmark suite
func (bs *BenchmarkSuite) RunBenchmarks() error {
	bs.logger.Printf("Starting performance benchmark suite...")
	bs.logger.Printf("Target counts: %v", bs.config.TargetCounts)
	bs.logger.Printf("Concurrency levels: %v", bs.config.ConcurrencyLevels)
	bs.logger.Printf("Duration per test: %v", bs.config.Duration)
	bs.logger.Printf("Iterations per test: %d", bs.config.Iterations)

	totalTests := len(bs.config.TargetCounts) * len(bs.config.ConcurrencyLevels) * bs.config.Iterations
	currentTest := 0

	for _, targetCount := range bs.config.TargetCounts {
		for _, concurrency := range bs.config.ConcurrencyLevels {
			// Skip invalid combinations
			if concurrency > targetCount {
				continue
			}

			for iteration := 0; iteration < bs.config.Iterations; iteration++ {
				currentTest++
				bs.logger.Printf("Running benchmark %d/%d: %d targets, concurrency %d, iteration %d",
					currentTest, totalTests, targetCount, concurrency, iteration+1)

				result, err := bs.runSingleBenchmark(targetCount, concurrency)
				if err != nil {
					bs.logger.Printf("Benchmark failed: %v", err)
					continue
				}

				bs.mutex.Lock()
				bs.results = append(bs.results, *result)
				bs.mutex.Unlock()

				// Brief pause between tests to allow system to stabilize
				time.Sleep(2 * time.Second)
			}
		}
	}

	bs.logger.Printf("Benchmark suite completed. Total results: %d", len(bs.results))
	return nil
}

// runSingleBenchmark executes a single benchmark test
func (bs *BenchmarkSuite) runSingleBenchmark(targetCount, concurrency int) (*BenchmarkResult, error) {
	// Create test targets
	targets := bs.createTestTargets(targetCount)

	// Create storage for benchmark
	storage := NewInMemoryStorage(MonitoringConfig{
		Targets:  make([]string, len(targets)),
		Interval: 1 * time.Second,
	})

	for i, target := range targets {
		storage.GetSession().Config.Targets[i] = target.URL
	}

	// Create optimized collector
	collector := NewOptimizedCollector(bs.performanceManager, 10*time.Second, bs.logger)

	// Warmup phase
	bs.logger.Printf("Warmup phase (%v)...", bs.config.WarmupDuration)
	warmupCtx, warmupCancel := context.WithTimeout(context.Background(), bs.config.WarmupDuration)
	bs.runBenchmarkLoad(warmupCtx, targets, collector, storage, concurrency)
	warmupCancel()

	// Clear warmup data by creating new storage
	storage = NewInMemoryStorage(MonitoringConfig{
		Targets:  make([]string, len(targets)),
		Interval: 1 * time.Second,
	})

	for i, target := range targets {
		storage.GetSession().Config.Targets[i] = target.URL
	}

	// Reset performance metrics
	bs.performanceManager.metrics.mutex.Lock()
	bs.performanceManager.metrics.TotalRequests = 0
	bs.performanceManager.metrics.AverageResponseTime = 0
	bs.performanceManager.metrics.mutex.Unlock()

	// Measurement phase
	bs.logger.Printf("Measurement phase (%v)...", bs.config.Duration)
	
	// Record initial system state
	var initialMemStats runtime.MemStats
	runtime.ReadMemStats(&initialMemStats)
	initialGoroutines := runtime.NumGoroutine()

	measurementCtx, measurementCancel := context.WithTimeout(context.Background(), bs.config.Duration)
	startTime := time.Now()
	
	latencies := bs.runBenchmarkLoad(measurementCtx, targets, collector, storage, concurrency)
	
	endTime := time.Now()
	measurementCancel()

	// Record final system state
	var finalMemStats runtime.MemStats
	runtime.ReadMemStats(&finalMemStats)
	finalGoroutines := runtime.NumGoroutine()

	// Calculate results
	actualDuration := endTime.Sub(startTime)
	totalRequests := int64(len(latencies))
	requestsPerSecond := float64(totalRequests) / actualDuration.Seconds()

	// Calculate latency statistics
	avgLatency, minLatency, maxLatency, p95Latency, p99Latency := bs.calculateLatencyStats(latencies)

	// Calculate error rate
	errorCount := 0
	for _, data := range storage.GetSession().Data {
		if data.Status != "success" {
			errorCount++
		}
	}
	errorRate := float64(errorCount) / float64(totalRequests) * 100

	// Calculate memory usage
	memoryUsageMB := float64(finalMemStats.Alloc-initialMemStats.Alloc) / 1024 / 1024

	result := &BenchmarkResult{
		TargetCount:       targetCount,
		ConcurrencyLevel:  concurrency,
		Duration:          actualDuration,
		TotalRequests:     totalRequests,
		RequestsPerSecond: requestsPerSecond,
		AverageLatency:    avgLatency,
		MinLatency:        minLatency,
		MaxLatency:        maxLatency,
		P95Latency:        p95Latency,
		P99Latency:        p99Latency,
		ErrorRate:         errorRate,
		MemoryUsageMB:     memoryUsageMB,
		GoroutineCount:    finalGoroutines - initialGoroutines,
		CPUUsagePercent:   0, // CPU usage calculation would require additional monitoring
	}

	bs.logger.Printf("Benchmark result: %.2f req/s, avg latency: %v, error rate: %.2f%%, memory: %.2f MB",
		requestsPerSecond, avgLatency, errorRate, memoryUsageMB)

	return result, nil
}

// runBenchmarkLoad executes the benchmark load and returns latency measurements
func (bs *BenchmarkSuite) runBenchmarkLoad(ctx context.Context, targets []ParsedTargetConfig, collector *OptimizedCollector, storage DataStorage, concurrency int) []time.Duration {
	var latencies []time.Duration
	var latencyMutex sync.Mutex

	// Create semaphore for concurrency control
	semaphore := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	// Start load generation
	for {
		select {
		case <-ctx.Done():
			// Wait for all goroutines to complete
			wg.Wait()
			return latencies

		default:
			for _, target := range targets {
				select {
				case <-ctx.Done():
					wg.Wait()
					return latencies
				case semaphore <- struct{}{}:
					wg.Add(1)
					go func(t ParsedTargetConfig) {
						defer wg.Done()
						defer func() { <-semaphore }()

						startTime := time.Now()
						data, err := collector.CollectData(t.URL)
						latency := time.Since(startTime)

						// Record latency
						latencyMutex.Lock()
						latencies = append(latencies, latency)
						latencyMutex.Unlock()

						// Store data
						if data != nil {
							storage.Store(data)
						} else if err != nil {
							// Create error data point
							storage.Store(&MonitoringData{
								Timestamp: time.Now(),
								Target:    t.URL,
								Status:    "failed",
								Error:     err.Error(),
							})
						}
					}(target)
				}
			}
		}
	}
}

// createTestTargets creates test targets for benchmarking
func (bs *BenchmarkSuite) createTestTargets(count int) []ParsedTargetConfig {
	targets := make([]ParsedTargetConfig, count)
	
	// Create a mix of HTTP and TCP targets for realistic testing
	httpTargets := []string{
		"https://httpbin.org/delay/0",
		"https://httpbin.org/status/200",
		"https://httpbin.org/get",
		"https://jsonplaceholder.typicode.com/posts/1",
		"https://api.github.com",
	}
	
	tcpTargets := []string{
		"google.com:80",
		"github.com:443",
		"stackoverflow.com:80",
	}

	for i := 0; i < count; i++ {
		if i%4 == 0 && len(tcpTargets) > 0 {
			// 25% TCP targets
			targets[i] = ParsedTargetConfig{
				URL:     tcpTargets[i%len(tcpTargets)],
				Timeout: 10 * time.Second,
			}
		} else {
			// 75% HTTP targets
			targets[i] = ParsedTargetConfig{
				URL:            httpTargets[i%len(httpTargets)],
				Timeout:        10 * time.Second,
				ExpectedStatus: 200,
			}
		}
	}

	return targets
}

// calculateLatencyStats calculates latency statistics from measurements
func (bs *BenchmarkSuite) calculateLatencyStats(latencies []time.Duration) (avg, min, max, p95, p99 time.Duration) {
	if len(latencies) == 0 {
		return 0, 0, 0, 0, 0
	}

	// Sort latencies for percentile calculations
	sortedLatencies := make([]time.Duration, len(latencies))
	copy(sortedLatencies, latencies)
	
	// Simple bubble sort (sufficient for benchmark data)
	for i := 0; i < len(sortedLatencies); i++ {
		for j := i + 1; j < len(sortedLatencies); j++ {
			if sortedLatencies[i] > sortedLatencies[j] {
				sortedLatencies[i], sortedLatencies[j] = sortedLatencies[j], sortedLatencies[i]
			}
		}
	}

	// Calculate statistics
	min = sortedLatencies[0]
	max = sortedLatencies[len(sortedLatencies)-1]

	// Calculate average
	var total time.Duration
	for _, latency := range latencies {
		total += latency
	}
	avg = total / time.Duration(len(latencies))

	// Calculate percentiles
	p95Index := int(float64(len(sortedLatencies)) * 0.95)
	p99Index := int(float64(len(sortedLatencies)) * 0.99)
	
	if p95Index >= len(sortedLatencies) {
		p95Index = len(sortedLatencies) - 1
	}
	if p99Index >= len(sortedLatencies) {
		p99Index = len(sortedLatencies) - 1
	}

	p95 = sortedLatencies[p95Index]
	p99 = sortedLatencies[p99Index]

	return avg, min, max, p95, p99
}

// GetResults returns all benchmark results
func (bs *BenchmarkSuite) GetResults() []BenchmarkResult {
	bs.mutex.RLock()
	defer bs.mutex.RUnlock()

	results := make([]BenchmarkResult, len(bs.results))
	copy(results, bs.results)
	return results
}

// GenerateReport generates a performance benchmark report
func (bs *BenchmarkSuite) GenerateReport() string {
	results := bs.GetResults()
	if len(results) == 0 {
		return "No benchmark results available"
	}

	report := fmt.Sprintf("Performance Benchmark Report\n")
	report += fmt.Sprintf("============================\n\n")
	report += fmt.Sprintf("Total tests run: %d\n", len(results))
	report += fmt.Sprintf("Test duration: %v\n", bs.config.Duration)
	report += fmt.Sprintf("Warmup duration: %v\n\n", bs.config.WarmupDuration)

	report += fmt.Sprintf("%-8s %-12s %-12s %-12s %-12s %-12s %-10s %-10s\n",
		"Targets", "Concurrency", "Req/Sec", "Avg Latency", "P95 Latency", "P99 Latency", "Error %", "Memory MB")
	report += fmt.Sprintf("%-8s %-12s %-12s %-12s %-12s %-12s %-10s %-10s\n",
		"-------", "-----------", "-------", "-----------", "-----------", "-----------", "-------", "---------")

	for _, result := range results {
		report += fmt.Sprintf("%-8d %-12d %-12.2f %-12s %-12s %-12s %-10.2f %-10.2f\n",
			result.TargetCount,
			result.ConcurrencyLevel,
			result.RequestsPerSecond,
			result.AverageLatency.String(),
			result.P95Latency.String(),
			result.P99Latency.String(),
			result.ErrorRate,
			result.MemoryUsageMB,
		)
	}

	// Add summary statistics
	report += fmt.Sprintf("\nSummary Statistics:\n")
	report += fmt.Sprintf("------------------\n")
	
	var totalReqPerSec, totalMemory float64
	var maxReqPerSec, minReqPerSec float64 = 0, float64(^uint(0) >> 1)
	
	for _, result := range results {
		totalReqPerSec += result.RequestsPerSecond
		totalMemory += result.MemoryUsageMB
		
		if result.RequestsPerSecond > maxReqPerSec {
			maxReqPerSec = result.RequestsPerSecond
		}
		if result.RequestsPerSecond < minReqPerSec {
			minReqPerSec = result.RequestsPerSecond
		}
	}
	
	avgReqPerSec := totalReqPerSec / float64(len(results))
	avgMemory := totalMemory / float64(len(results))
	
	report += fmt.Sprintf("Average requests/sec: %.2f\n", avgReqPerSec)
	report += fmt.Sprintf("Max requests/sec: %.2f\n", maxReqPerSec)
	report += fmt.Sprintf("Min requests/sec: %.2f\n", minReqPerSec)
	report += fmt.Sprintf("Average memory usage: %.2f MB\n", avgMemory)

	return report
}