# Performance Optimization Guide

This document describes the performance optimization features implemented in Netirk's enhanced network monitoring system.

## Overview

The performance optimization system provides comprehensive resource management, connection pooling, and memory optimization for long-running monitoring sessions. It includes three main components:

1. **PerformanceManager** - Manages HTTP connection pooling, concurrency control, and system resource monitoring
2. **OptimizedStorage** - Provides memory-efficient data storage with automatic rotation and buffering
3. **OptimizedCollector** - Implements rate-limited data collection with performance metrics

## Key Features

### 1. Connection Pooling and Resource Management

- **HTTP Connection Pooling**: Reuses HTTP connections to reduce overhead
- **Configurable Pool Sizes**: Adjustable connection pool sizes based on workload
- **Automatic Connection Cleanup**: Closes idle connections to free resources
- **Optimized Transport Settings**: Tuned timeouts and keep-alive settings

```go
// Example: Create performance manager with custom settings
config := helpers.PerformanceConfig{
    MaxConcurrentTargets: 50,
    ConnectionPoolSize:   20,
    MemoryLimitMB:       100,
    GCInterval:          5 * time.Minute,
    MetricsInterval:     30 * time.Second,
}
perfManager := helpers.NewPerformanceManager(config, logger)
```

### 2. Memory Management

- **Memory Usage Monitoring**: Tracks memory consumption in real-time
- **Automatic Garbage Collection**: Triggers GC when memory limits are exceeded
- **Data Buffer Management**: Implements buffering with automatic flushing
- **Data Rotation**: Automatically rotates old data to prevent memory leaks

```go
// Check memory usage
withinLimit, memoryUsage := perfManager.CheckMemoryUsage()
if !withinLimit {
    perfManager.ForceGarbageCollection()
}
```

### 3. Concurrency Control

- **Rate Limiting**: Controls the number of concurrent target checks
- **Semaphore-based Control**: Uses semaphores to limit resource usage
- **Graceful Degradation**: Continues operation even when some targets fail
- **Performance Metrics**: Tracks concurrent request counts and response times

```go
// Acquire slot for rate limiting
ctx := context.Background()
err := perfManager.AcquireSlot(ctx)
if err != nil {
    // Handle rate limiting
}
defer perfManager.ReleaseSlot()
```

### 4. Long-Running Session Optimization

- **Adaptive Configuration**: Adjusts settings for long-running sessions
- **Reduced Resource Usage**: Optimizes connection pools for extended operation
- **Periodic Cleanup**: Performs regular maintenance tasks
- **Background Monitoring**: Continuously monitors system health

```go
// Optimize for long-running sessions
if sessionDuration > 30*time.Minute {
    perfManager.OptimizeForLongRunning()
}
```

## Configuration Options

### PerformanceConfig

| Setting | Default | Description |
|---------|---------|-------------|
| MaxConcurrentTargets | 50 | Maximum concurrent target checks |
| ConnectionPoolSize | 20 | HTTP connection pool size |
| MemoryLimitMB | 100 | Memory limit in MB |
| GCInterval | 5 minutes | Garbage collection interval |
| MetricsInterval | 30 seconds | Performance metrics collection interval |

### OptimizedStorageConfig

| Setting | Default | Description |
|---------|---------|-------------|
| MaxBufferSize | 10,000 | Maximum data points in memory buffer |
| FlushInterval | 5 minutes | How often to flush buffer to storage |
| RotationSize | 50,000 | Data points before rotation |
| EnableCompression | false | Enable data compression |
| BackupEnabled | true | Enable automatic backups |

## Performance Metrics

The system tracks comprehensive performance metrics:

- **Request Metrics**: Total requests, requests per second, concurrent requests
- **Latency Metrics**: Average, minimum, maximum, P95, P99 response times
- **Resource Metrics**: Memory usage, goroutine count, connection statistics
- **Error Metrics**: Error rates, failure counts, recovery statistics

```go
// Get performance metrics
metrics := perfManager.GetMetrics()
fmt.Printf("Requests/sec: %.2f, Avg latency: %v, Memory: %.2f MB",
    metrics.RequestsPerSecond, metrics.AverageResponseTime, metrics.MemoryUsageMB)
```

## Usage Examples

### Basic Performance Optimization

```go
// Create performance manager
perfManager := helpers.NewPerformanceManager(
    helpers.DefaultPerformanceConfig(), 
    logger,
)
defer perfManager.Cleanup()

// Use optimized collector
collector := helpers.NewOptimizedCollector(perfManager, 30*time.Second, logger)
data, err := collector.CollectData("https://example.com")
```

### Long-Running Monitoring Session

```go
// Create optimized storage for long sessions
storage := helpers.NewOptimizedStorage(config, perfManager, logger)

// Optimize for long-running operation
perfManager.OptimizeForLongRunning()

// Run optimized collection
err := helpers.OptimizedCollectMetrics(targets, storage, perfManager, logger)
```

### Performance Monitoring

```go
// Start performance monitoring
go func() {
    ticker := time.NewTicker(60 * time.Second)
    defer ticker.Stop()
    
    for range ticker.C {
        perfManager.LogPerformanceStats()
        
        utilization := perfManager.GetResourceUtilization()
        for key, value := range utilization {
            log.Printf("%s: %v", key, value)
        }
    }
}()
```

## Benchmarking

The system includes comprehensive benchmarking capabilities:

```go
// Create benchmark suite
benchConfig := helpers.DefaultBenchmarkConfig()
benchSuite := helpers.NewBenchmarkSuite(benchConfig, perfManager, logger)

// Run benchmarks
err := benchSuite.RunBenchmarks()
if err != nil {
    log.Printf("Benchmark failed: %v", err)
}

// Generate report
report := benchSuite.GenerateReport()
fmt.Println(report)
```

## Best Practices

1. **Resource Limits**: Set appropriate memory and concurrency limits based on system capacity
2. **Long Sessions**: Use OptimizedStorage for sessions longer than 30 minutes
3. **Monitoring**: Enable performance monitoring for sessions with more than 10 targets
4. **Cleanup**: Always call Cleanup() on PerformanceManager when done
5. **Error Handling**: Handle rate limiting and memory pressure gracefully

## Troubleshooting

### High Memory Usage
- Reduce MaxBufferSize in storage configuration
- Decrease MemoryLimitMB to trigger more frequent GC
- Enable data rotation with smaller RotationSize

### Poor Performance
- Increase ConnectionPoolSize for better connection reuse
- Adjust MaxConcurrentTargets based on system capacity
- Use OptimizeForLongRunning() for extended sessions

### Connection Issues
- Check connection pool settings
- Verify timeout configurations
- Monitor connection cleanup logs

## Integration with Monitor Command

The monitor command automatically uses performance optimizations when:

- Session duration exceeds 30 minutes (uses OptimizedStorage)
- More than 10 targets are monitored (enables performance monitoring)
- More than 5 targets are monitored (uses OptimizedCollectMetrics)

The optimizations are transparent to users and activate automatically based on workload characteristics.