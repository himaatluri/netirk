package helpers

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httptrace"
	"strings"
	"sync"
	"time"
)

// OptimizedCollector implements DataCollector with performance optimizations
type OptimizedCollector struct {
	performanceManager *PerformanceManager
	errorHandler       *ErrorHandler
	recoveryManager    *RecoveryManager
	logger             *log.Logger
	defaultTimeout     time.Duration
}

// NewOptimizedCollector creates a new optimized data collector
func NewOptimizedCollector(perfManager *PerformanceManager, timeout time.Duration, logger *log.Logger) *OptimizedCollector {
	if logger == nil {
		logger = log.Default()
	}

	errorHandler := NewErrorHandler(logger)
	recoveryManager := NewRecoveryManager(errorHandler, logger)

	return &OptimizedCollector{
		performanceManager: perfManager,
		errorHandler:       errorHandler,
		recoveryManager:    recoveryManager,
		logger:             logger,
		defaultTimeout:     timeout,
	}
}

// SetTimeout updates the collector's default timeout
func (c *OptimizedCollector) SetTimeout(timeout time.Duration) {
	c.defaultTimeout = timeout
}

// CollectData collects monitoring data for a single target with performance optimization
func (c *OptimizedCollector) CollectData(target string) (*MonitoringData, error) {
	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), c.defaultTimeout)
	defer cancel()

	// Acquire slot for rate limiting
	if err := c.performanceManager.AcquireSlot(ctx); err != nil {
		return nil, CreateTimeoutError("rate_limit", c.defaultTimeout)
	}
	defer c.performanceManager.ReleaseSlot()

	// Check if target should be skipped due to recent failures
	if c.recoveryManager.ShouldSkipTarget(target) {
		status := c.recoveryManager.GetTargetStatus(target)
		return &MonitoringData{
			Timestamp: time.Now(),
			Target:    target,
			Status:    "skipped",
			Error:     fmt.Sprintf("Target skipped due to %s status", status),
		}, nil
	}

	data := &MonitoringData{
		Timestamp: time.Now(),
		Target:    target,
		Status:    "unknown",
	}

	var err error
	var collectionErr *NetirkError

	// Record start time for performance metrics
	startTime := time.Now()

	// Determine if this is HTTP/HTTPS or TCP and collect data
	if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") {
		data, err = c.collectHTTPDataOptimized(ctx, target, data)
	} else {
		data, err = c.collectTCPDataOptimized(ctx, target, data)
	}

	// Record performance metrics
	responseTime := time.Since(startTime)
	c.performanceManager.RecordRequest(responseTime)

	// Handle collection errors
	if err != nil {
		collectionErr = c.errorHandler.HandleError(err, fmt.Sprintf("collect_%s", target))
		collectionErr.Target = target

		// Update recovery manager with failure
		c.recoveryManager.HandleTargetFailure(target, collectionErr)

		// Update data with error information
		data.Status = "failed"
		data.Error = collectionErr.GetUserFriendlyMessage()

		return data, collectionErr
	}

	// Handle successful collection
	if data.Status == "success" {
		c.recoveryManager.HandleTargetSuccess(target)
	}

	return data, nil
}

// collectHTTPDataOptimized collects data for HTTP/HTTPS targets using optimized client
func (c *OptimizedCollector) collectHTTPDataOptimized(ctx context.Context, target string, data *MonitoringData) (*MonitoringData, error) {
	var start, dnsStart, connectStart, tlsStart time.Time
	var dnsTime, connectTime, tlsTime, firstByteTime time.Duration
	var sslInfo *SSLData

	// Use the optimized HTTP client from performance manager
	client := c.performanceManager.GetOptimizedHTTPClient()

	// Create HTTP request with context
	req, err := http.NewRequestWithContext(ctx, "GET", target, nil)
	if err != nil {
		return data, CreateNetworkError(target, err, "http_request_creation")
	}

	// Setup HTTP tracing for detailed performance metrics
	trace := &httptrace.ClientTrace{
		DNSStart: func(_ httptrace.DNSStartInfo) {
			dnsStart = time.Now()
		},
		DNSDone: func(_ httptrace.DNSDoneInfo) {
			dnsTime = time.Since(dnsStart)
		},
		ConnectStart: func(_, _ string) {
			connectStart = time.Now()
		},
		ConnectDone: func(_, _ string, _ error) {
			connectTime = time.Since(connectStart)
		},
		TLSHandshakeStart: func() {
			tlsStart = time.Now()
		},
		TLSHandshakeDone: func(cs tls.ConnectionState, tlsErr error) {
			tlsTime = time.Since(tlsStart)

			// Handle TLS errors
			if tlsErr != nil {
				sslInfo = &SSLData{
					IsValid: false,
					Error:   tlsErr.Error(),
				}
				return
			}

			// Collect SSL certificate information
			if len(cs.PeerCertificates) > 0 {
				cert := cs.PeerCertificates[0]
				daysToExpiry := int(time.Until(cert.NotAfter).Hours() / 24)
				sslInfo = &SSLData{
					ExpiryDate:   cert.NotAfter,
					DaysToExpiry: daysToExpiry,
					Issuer:       cert.Issuer.CommonName,
					IsValid:      time.Now().Before(cert.NotAfter),
				}

				// Check for SSL certificate warnings
				if daysToExpiry <= 30 && daysToExpiry > 0 {
					c.logger.Printf("SSL certificate for %s expires in %d days", target, daysToExpiry)
				}
			}
		},
		GotFirstResponseByte: func() {
			firstByteTime = time.Since(start)
		},
	}

	// Add trace to request context
	req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))

	// Record start time and make request
	start = time.Now()
	resp, err := client.Do(req)
	totalTime := time.Since(start)

	// Handle response and errors
	data.ResponseTime = totalTime

	if err != nil {
		// Classify the error for better handling
		if ctx.Err() == context.DeadlineExceeded {
			return data, CreateTimeoutError("http_request", c.defaultTimeout)
		}
		return data, CreateNetworkError(target, err, "http_request")
	}

	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			c.logger.Printf("Warning: failed to close response body for %s: %v", target, closeErr)
		}
	}()

	data.StatusCode = resp.StatusCode

	// Determine success/failure based on status code
	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		data.Status = "success"
	} else {
		data.Status = "failed"
		data.Error = fmt.Sprintf("HTTP %d %s", resp.StatusCode, http.StatusText(resp.StatusCode))
	}

	// Add trace data if we have timing information
	if dnsTime > 0 || connectTime > 0 || tlsTime > 0 || firstByteTime > 0 {
		data.TraceData = &NetworkTrace{
			DNSTime:       dnsTime,
			ConnectTime:   connectTime,
			TLSTime:       tlsTime,
			FirstByteTime: firstByteTime,
		}
	}

	// Add SSL information if available
	if sslInfo != nil {
		data.SSLInfo = sslInfo
	}

	return data, nil
}

// collectTCPDataOptimized collects data for TCP targets with optimization
func (c *OptimizedCollector) collectTCPDataOptimized(ctx context.Context, target string, data *MonitoringData) (*MonitoringData, error) {
	start := time.Now()

	// Use optimized dialer settings
	dialer := &net.Dialer{
		Timeout:   c.defaultTimeout,
		KeepAlive: 30 * time.Second,
	}

	// Attempt TCP connection with context
	conn, err := dialer.DialContext(ctx, "tcp", target)
	totalTime := time.Since(start)

	data.ResponseTime = totalTime

	if err != nil {
		// Classify the error for better handling
		if ctx.Err() == context.DeadlineExceeded {
			return data, CreateTimeoutError("tcp_connection", c.defaultTimeout)
		}
		return data, CreateNetworkError(target, err, "tcp_connection")
	}

	defer func() {
		if closeErr := conn.Close(); closeErr != nil {
			c.logger.Printf("Warning: failed to close TCP connection for %s: %v", target, closeErr)
		}
	}()

	data.Status = "success"
	return data, nil
}

// OptimizedCollectMetrics orchestrates optimized data collection for multiple targets
func OptimizedCollectMetrics(targets []ParsedTargetConfig, storage DataStorage, perfManager *PerformanceManager, logger *log.Logger) error {
	if len(targets) == 0 {
		return fmt.Errorf("no targets provided for monitoring")
	}

	if logger == nil {
		logger = log.Default()
	}

	// Create optimized collector
	collector := NewOptimizedCollector(perfManager, 30*time.Second, logger)

	// Create channels for results and errors
	results := make(chan *MonitoringData, len(targets))
	errors := make(chan error, len(targets))

	// Create context with global timeout (removed unused ctx variable)

	// Use WaitGroup for better goroutine management
	var wg sync.WaitGroup

	// Launch goroutines for concurrent monitoring with rate limiting
	for _, target := range targets {
		wg.Add(1)
		go func(t ParsedTargetConfig) {
			defer wg.Done()

			// Set target-specific timeout if available
			if t.Timeout > 0 {
				collector.SetTimeout(t.Timeout)
			}

			data, err := collector.CollectData(t.URL)
			if err != nil {
				// Log the error but don't fail the entire collection
				logger.Printf("Collection error for %s: %v", t.URL, FormatErrorForUser(err))

				// Still send the data (which contains error information)
				if data != nil {
					results <- data
				} else {
					// Create minimal error data if data is nil
					results <- &MonitoringData{
						Timestamp: time.Now(),
						Target:    t.URL,
						Status:    "failed",
						Error:     FormatErrorForUser(err),
					}
				}
				return
			}

			// Validate against expected status if specified
			if t.ExpectedStatus != 0 && data.StatusCode != 0 && data.StatusCode != t.ExpectedStatus {
				data.Status = "failed"
				data.Error = fmt.Sprintf("Expected status %d, got %d", t.ExpectedStatus, data.StatusCode)
			}

			results <- data
		}(target)
	}

	// Wait for all goroutines to complete
	go func() {
		wg.Wait()
		close(results)
		close(errors)
	}()

	// Collect results with improved error handling
	var collectionErrors []error
	successCount := 0

	for data := range results {
		// Store data with error handling
		if err := storage.Store(data); err != nil {
			collectionErrors = append(collectionErrors,
				fmt.Errorf("failed to store data for %s: %w", data.Target, err))
		} else {
			successCount++
		}
	}

	// Log collection summary with performance metrics
	metrics := perfManager.GetMetrics()
	logger.Printf("Optimized collection completed: %d/%d targets successful, %d errors, avg response time: %v",
		successCount, len(targets), len(collectionErrors), metrics.AverageResponseTime)

	// Return aggregated errors if any occurred, but don't fail completely
	if len(collectionErrors) > 0 {
		// Log errors but continue operation (graceful degradation)
		for _, err := range collectionErrors {
			logger.Printf("Collection error: %v", FormatErrorForUser(err))
		}

		// Only fail if all targets failed
		if successCount == 0 {
			return fmt.Errorf("all targets failed during optimized collection")
		}

		// Return a warning error that indicates partial success
		return fmt.Errorf("optimized collection completed with %d errors out of %d targets",
			len(collectionErrors), len(targets))
	}

	return nil
}