package helpers

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"runtime"
	"sync"
	"time"
)

// PerformanceConfig holds configuration for performance optimization
type PerformanceConfig struct {
	MaxConcurrentTargets int           // Maximum number of concurrent target checks
	ConnectionPoolSize   int           // Size of HTTP connection pool
	MemoryLimitMB       int           // Memory limit in MB for data storage
	GCInterval          time.Duration // Garbage collection interval
	MetricsInterval     time.Duration // Performance metrics collection interval
}

// DefaultPerformanceConfig returns sensible defaults for performance configuration
func DefaultPerformanceConfig() PerformanceConfig {
	return PerformanceConfig{
		MaxConcurrentTargets: 50,
		ConnectionPoolSize:   20,
		MemoryLimitMB:       100,
		GCInterval:          5 * time.Minute,
		MetricsInterval:     30 * time.Second,
	}
}

// PerformanceManager manages performance optimization for monitoring operations
type PerformanceManager struct {
	config          PerformanceConfig
	httpClient      *http.Client
	transport       *http.Transport
	semaphore       chan struct{}
	metrics         *PerformanceMetrics
	gcTicker        *time.Ticker
	metricsTicker   *time.Ticker
	ctx             context.Context
	cancel          context.CancelFunc
	mutex           sync.RWMutex
	logger          *log.Logger
}

// PerformanceMetrics tracks performance statistics
type PerformanceMetrics struct {
	TotalRequests       int64         `json:"total_requests"`
	ConcurrentRequests  int64         `json:"concurrent_requests"`
	AverageResponseTime time.Duration `json:"average_response_time"`
	MemoryUsageMB      float64       `json:"memory_usage_mb"`
	GoroutineCount     int           `json:"goroutine_count"`
	ConnectionsActive  int           `json:"connections_active"`
	ConnectionsIdle    int           `json:"connections_idle"`
	LastGCTime         time.Time     `json:"last_gc_time"`
	mutex              sync.RWMutex  `json:"-"`
}

// NewPerformanceManager creates a new performance manager with optimized settings
func NewPerformanceManager(config PerformanceConfig, logger *log.Logger) *PerformanceManager {
	if logger == nil {
		logger = log.Default()
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Create optimized HTTP transport with connection pooling
	transport := &http.Transport{
		MaxIdleConns:        config.ConnectionPoolSize,
		MaxIdleConnsPerHost: config.ConnectionPoolSize / 4,
		MaxConnsPerHost:     config.ConnectionPoolSize / 2,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		DisableKeepAlives:   false,
		DisableCompression:  false,
		// Custom dialer with optimized settings
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
	}

	// Create HTTP client with optimized transport
	httpClient := &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}

	pm := &PerformanceManager{
		config:     config,
		httpClient: httpClient,
		transport:  transport,
		semaphore:  make(chan struct{}, config.MaxConcurrentTargets),
		metrics:    &PerformanceMetrics{},
		ctx:        ctx,
		cancel:     cancel,
		logger:     logger,
	}

	// Start background performance monitoring
	pm.startBackgroundTasks()

	return pm
}

// GetOptimizedHTTPClient returns the performance-optimized HTTP client
func (pm *PerformanceManager) GetOptimizedHTTPClient() *http.Client {
	return pm.httpClient
}

// AcquireSlot acquires a slot for concurrent execution (rate limiting)
func (pm *PerformanceManager) AcquireSlot(ctx context.Context) error {
	select {
	case pm.semaphore <- struct{}{}:
		pm.updateConcurrentRequests(1)
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// ReleaseSlot releases a slot after execution completion
func (pm *PerformanceManager) ReleaseSlot() {
	select {
	case <-pm.semaphore:
		pm.updateConcurrentRequests(-1)
	default:
		pm.logger.Printf("Warning: attempted to release slot when none were acquired")
	}
}

// RecordRequest records performance metrics for a completed request
func (pm *PerformanceManager) RecordRequest(responseTime time.Duration) {
	pm.metrics.mutex.Lock()
	defer pm.metrics.mutex.Unlock()

	pm.metrics.TotalRequests++
	
	// Calculate rolling average response time
	if pm.metrics.TotalRequests == 1 {
		pm.metrics.AverageResponseTime = responseTime
	} else {
		// Exponential moving average with alpha = 0.1
		alpha := 0.1
		pm.metrics.AverageResponseTime = time.Duration(
			float64(pm.metrics.AverageResponseTime)*(1-alpha) + 
			float64(responseTime)*alpha,
		)
	}
}

// GetMetrics returns current performance metrics
func (pm *PerformanceManager) GetMetrics() PerformanceMetrics {
	pm.metrics.mutex.RLock()
	defer pm.metrics.mutex.RUnlock()

	// Create a copy to avoid race conditions
	return PerformanceMetrics{
		TotalRequests:       pm.metrics.TotalRequests,
		ConcurrentRequests:  pm.metrics.ConcurrentRequests,
		AverageResponseTime: pm.metrics.AverageResponseTime,
		MemoryUsageMB:      pm.metrics.MemoryUsageMB,
		GoroutineCount:     pm.metrics.GoroutineCount,
		ConnectionsActive:  pm.metrics.ConnectionsActive,
		ConnectionsIdle:    pm.metrics.ConnectionsIdle,
		LastGCTime:         pm.metrics.LastGCTime,
	}
}

// CheckMemoryUsage checks if memory usage is within limits
func (pm *PerformanceManager) CheckMemoryUsage() (bool, float64) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	
	memoryUsageMB := float64(m.Alloc) / 1024 / 1024
	pm.updateMemoryUsage(memoryUsageMB)
	
	return memoryUsageMB < float64(pm.config.MemoryLimitMB), memoryUsageMB
}

// ForceGarbageCollection triggers garbage collection if memory usage is high
func (pm *PerformanceManager) ForceGarbageCollection() {
	withinLimit, memoryUsage := pm.CheckMemoryUsage()
	
	if !withinLimit {
		pm.logger.Printf("Memory usage (%.2f MB) exceeds limit (%d MB), forcing garbage collection", 
			memoryUsage, pm.config.MemoryLimitMB)
		
		runtime.GC()
		runtime.GC() // Double GC for better cleanup
		
		pm.metrics.mutex.Lock()
		pm.metrics.LastGCTime = time.Now()
		pm.metrics.mutex.Unlock()
		
		// Check memory usage after GC
		_, newMemoryUsage := pm.CheckMemoryUsage()
		pm.logger.Printf("Memory usage after GC: %.2f MB", newMemoryUsage)
	}
}

// OptimizeForLongRunning optimizes settings for long-running monitoring sessions
func (pm *PerformanceManager) OptimizeForLongRunning() {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	// Reduce connection pool size for long-running sessions to save memory
	pm.transport.MaxIdleConns = pm.config.ConnectionPoolSize / 2
	pm.transport.MaxIdleConnsPerHost = pm.config.ConnectionPoolSize / 8
	
	// Reduce idle connection timeout to free up resources faster
	pm.transport.IdleConnTimeout = 60 * time.Second
	
	pm.logger.Printf("Optimized settings for long-running session")
}

// Cleanup performs cleanup of resources and stops background tasks
func (pm *PerformanceManager) Cleanup() {
	pm.logger.Printf("Cleaning up performance manager resources...")
	
	// Cancel background tasks
	pm.cancel()
	
	// Stop tickers
	if pm.gcTicker != nil {
		pm.gcTicker.Stop()
	}
	if pm.metricsTicker != nil {
		pm.metricsTicker.Stop()
	}
	
	// Close idle connections
	pm.transport.CloseIdleConnections()
	
	// Force final garbage collection
	runtime.GC()
	
	pm.logger.Printf("Performance manager cleanup completed")
}

// startBackgroundTasks starts background performance monitoring tasks
func (pm *PerformanceManager) startBackgroundTasks() {
	// Start garbage collection ticker
	pm.gcTicker = time.NewTicker(pm.config.GCInterval)
	go pm.gcWorker()
	
	// Start metrics collection ticker
	pm.metricsTicker = time.NewTicker(pm.config.MetricsInterval)
	go pm.metricsWorker()
}

// gcWorker runs periodic garbage collection
func (pm *PerformanceManager) gcWorker() {
	for {
		select {
		case <-pm.gcTicker.C:
			pm.ForceGarbageCollection()
		case <-pm.ctx.Done():
			return
		}
	}
}

// metricsWorker collects performance metrics periodically
func (pm *PerformanceManager) metricsWorker() {
	for {
		select {
		case <-pm.metricsTicker.C:
			pm.collectSystemMetrics()
		case <-pm.ctx.Done():
			return
		}
	}
}

// collectSystemMetrics collects system-level performance metrics
func (pm *PerformanceManager) collectSystemMetrics() {
	pm.metrics.mutex.Lock()
	defer pm.metrics.mutex.Unlock()

	// Update goroutine count
	pm.metrics.GoroutineCount = runtime.NumGoroutine()
	
	// Update memory usage
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	pm.metrics.MemoryUsageMB = float64(m.Alloc) / 1024 / 1024
	
	// Update connection statistics if available
	// Note: Go's http.Transport doesn't expose connection stats directly,
	// so we track what we can through our own metrics
}

// updateConcurrentRequests updates the concurrent request counter
func (pm *PerformanceManager) updateConcurrentRequests(delta int64) {
	pm.metrics.mutex.Lock()
	defer pm.metrics.mutex.Unlock()
	pm.metrics.ConcurrentRequests += delta
}

// updateMemoryUsage updates the memory usage metric
func (pm *PerformanceManager) updateMemoryUsage(memoryMB float64) {
	pm.metrics.mutex.Lock()
	defer pm.metrics.mutex.Unlock()
	pm.metrics.MemoryUsageMB = memoryMB
}

// GetResourceUtilization returns current resource utilization summary
func (pm *PerformanceManager) GetResourceUtilization() map[string]interface{} {
	metrics := pm.GetMetrics()
	
	return map[string]interface{}{
		"memory_usage_mb":     metrics.MemoryUsageMB,
		"memory_limit_mb":     pm.config.MemoryLimitMB,
		"memory_utilization":  fmt.Sprintf("%.1f%%", (metrics.MemoryUsageMB/float64(pm.config.MemoryLimitMB))*100),
		"goroutine_count":     metrics.GoroutineCount,
		"concurrent_requests": metrics.ConcurrentRequests,
		"max_concurrent":      pm.config.MaxConcurrentTargets,
		"total_requests":      metrics.TotalRequests,
		"avg_response_time":   metrics.AverageResponseTime.String(),
		"last_gc_time":        metrics.LastGCTime.Format("15:04:05"),
	}
}

// LogPerformanceStats logs current performance statistics
func (pm *PerformanceManager) LogPerformanceStats() {
	utilization := pm.GetResourceUtilization()
	
	pm.logger.Printf("Performance Stats - Memory: %s, Goroutines: %d, Concurrent: %d/%d, Avg Response: %s",
		utilization["memory_utilization"],
		utilization["goroutine_count"],
		utilization["concurrent_requests"],
		utilization["max_concurrent"],
		utilization["avg_response_time"],
	)
}