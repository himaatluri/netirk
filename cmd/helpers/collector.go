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
	"time"
)

// NetworkCollector implements DataCollector interface
type NetworkCollector struct {
	timeout         time.Duration
	client          *http.Client
	errorHandler    *ErrorHandler
	recoveryManager *RecoveryManager
}

// NewNetworkCollector creates a new network data collector
func NewNetworkCollector(timeout time.Duration) *NetworkCollector {
	errorHandler := NewErrorHandler(log.Default())
	recoveryManager := NewRecoveryManager(errorHandler, log.Default())
	
	return &NetworkCollector{
		timeout:         timeout,
		client: &http.Client{
			Timeout: timeout,
		},
		errorHandler:    errorHandler,
		recoveryManager: recoveryManager,
	}
}

// NewNetworkCollectorWithRecovery creates a new network data collector with custom error handling
func NewNetworkCollectorWithRecovery(timeout time.Duration, errorHandler *ErrorHandler, recoveryManager *RecoveryManager) *NetworkCollector {
	return &NetworkCollector{
		timeout:         timeout,
		client: &http.Client{
			Timeout: timeout,
		},
		errorHandler:    errorHandler,
		recoveryManager: recoveryManager,
	}
}

// SetTimeout updates the collector's timeout
func (c *NetworkCollector) SetTimeout(timeout time.Duration) {
	c.timeout = timeout
	c.client.Timeout = timeout
}

// CollectData collects monitoring data for a single target with error handling
func (c *NetworkCollector) CollectData(target string) (*MonitoringData, error) {
	// Check if target should be skipped due to recent failures
	if c.recoveryManager != nil && c.recoveryManager.ShouldSkipTarget(target) {
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

	// Determine if this is HTTP/HTTPS or TCP and collect data
	if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") {
		data, err = c.collectHTTPDataWithRecovery(target, data)
	} else {
		data, err = c.collectTCPDataWithRecovery(target, data)
	}

	// Handle collection errors
	if err != nil {
		collectionErr = c.errorHandler.HandleError(err, fmt.Sprintf("collect_%s", target))
		collectionErr.Target = target
		
		// Update recovery manager with failure
		if c.recoveryManager != nil {
			c.recoveryManager.HandleTargetFailure(target, collectionErr)
		}
		
		// Update data with error information
		data.Status = "failed"
		data.Error = collectionErr.GetUserFriendlyMessage()
		
		return data, collectionErr
	}

	// Handle successful collection
	if c.recoveryManager != nil && data.Status == "success" {
		c.recoveryManager.HandleTargetSuccess(target)
	}

	return data, nil
}

// collectHTTPDataWithRecovery collects data for HTTP/HTTPS targets with error handling
func (c *NetworkCollector) collectHTTPDataWithRecovery(target string, data *MonitoringData) (*MonitoringData, error) {
	// Use context with timeout for better error handling
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()
	
	return c.collectHTTPDataWithContext(ctx, target, data)
}

// collectHTTPDataWithContext collects data for HTTP/HTTPS targets with detailed tracing and context
func (c *NetworkCollector) collectHTTPDataWithContext(ctx context.Context, target string, data *MonitoringData) (*MonitoringData, error) {
	var start, dnsStart, connectStart, tlsStart time.Time
	var dnsTime, connectTime, tlsTime, firstByteTime time.Duration
	var sslInfo *SSLData

	// Create HTTP request with tracing and context
	req, err := http.NewRequestWithContext(ctx, "GET", target, nil)
	if err != nil {
		return data, CreateNetworkError(target, err, "http_request_creation")
	}

	// Setup HTTP tracing
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
					log.Printf("SSL certificate for %s expires in %d days", target, daysToExpiry)
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
	resp, err := c.client.Do(req)
	totalTime := time.Since(start)

	// Handle response and errors
	data.ResponseTime = totalTime
	
	if err != nil {
		// Classify the error for better handling
		if ctx.Err() == context.DeadlineExceeded {
			return data, CreateTimeoutError("http_request", c.timeout)
		}
		return data, CreateNetworkError(target, err, "http_request")
	}
	
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			log.Printf("Warning: failed to close response body for %s: %v", target, closeErr)
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

// collectTCPDataWithRecovery collects data for TCP targets with error handling
func (c *NetworkCollector) collectTCPDataWithRecovery(target string, data *MonitoringData) (*MonitoringData, error) {
	// Use context with timeout for better error handling
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()
	
	return c.collectTCPDataWithContext(ctx, target, data)
}

// collectTCPDataWithContext collects data for TCP targets with context and error handling
func (c *NetworkCollector) collectTCPDataWithContext(ctx context.Context, target string, data *MonitoringData) (*MonitoringData, error) {
	start := time.Now()

	// Attempt TCP connection with context
	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, "tcp", target)
	totalTime := time.Since(start)

	data.ResponseTime = totalTime

	if err != nil {
		// Classify the error for better handling
		if ctx.Err() == context.DeadlineExceeded {
			return data, CreateTimeoutError("tcp_connection", c.timeout)
		}
		return data, CreateNetworkError(target, err, "tcp_connection")
	}

	defer func() {
		if closeErr := conn.Close(); closeErr != nil {
			log.Printf("Warning: failed to close TCP connection for %s: %v", target, closeErr)
		}
	}()
	
	data.Status = "success"
	return data, nil
}

// CollectMetrics is the main function that orchestrates data collection for multiple targets with error handling
func CollectMetrics(targets []ParsedTargetConfig, storage DataStorage) error {
	return CollectMetricsWithRecovery(targets, storage, nil, nil)
}

// CollectMetricsWithRecovery orchestrates data collection with comprehensive error handling and recovery
func CollectMetricsWithRecovery(targets []ParsedTargetConfig, storage DataStorage, errorHandler *ErrorHandler, recoveryManager *RecoveryManager) error {
	if len(targets) == 0 {
		return fmt.Errorf("no targets provided for monitoring")
	}

	// Initialize error handling if not provided
	if errorHandler == nil {
		errorHandler = NewErrorHandler(log.Default())
	}
	if recoveryManager == nil {
		recoveryManager = NewRecoveryManager(errorHandler, log.Default())
	}

	// Create collectors for concurrent execution
	results := make(chan *MonitoringData, len(targets))
	errors := make(chan error, len(targets))
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second) // Global timeout
	defer cancel()

	// Launch goroutines for concurrent monitoring
	for _, target := range targets {
		go func(t ParsedTargetConfig) {
			collector := NewNetworkCollectorWithRecovery(t.Timeout, errorHandler, recoveryManager)
			
			data, err := collector.CollectData(t.URL)
			if err != nil {
				// Log the error but don't fail the entire collection
				log.Printf("Collection error for %s: %v", t.URL, FormatErrorForUser(err))
				
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

	// Collect results from all goroutines with improved error handling
	var collectionErrors []error
	successCount := 0
	
	for i := 0; i < len(targets); i++ {
		select {
		case data := <-results:
			// Attempt to store data with recovery
			storeErr := recoveryManager.RecoverStorageOperation(func() error {
				return storage.Store(data)
			}, "store_monitoring_data")
			
			if storeErr != nil {
				collectionErrors = append(collectionErrors, 
					fmt.Errorf("failed to store data for %s: %w", data.Target, storeErr))
			} else {
				successCount++
			}
			
		case err := <-errors:
			collectionErrors = append(collectionErrors, err)
			
		case <-ctx.Done():
			collectionErrors = append(collectionErrors, 
				CreateTimeoutError("collection_timeout", 60*time.Second))
		}
	}

	// Log collection summary
	log.Printf("Collection completed: %d/%d targets successful, %d errors", 
		successCount, len(targets), len(collectionErrors))

	// Return aggregated errors if any occurred, but don't fail completely
	if len(collectionErrors) > 0 {
		// Log errors but continue operation (graceful degradation)
		for _, err := range collectionErrors {
			log.Printf("Collection error: %v", FormatErrorForUser(err))
		}
		
		// Only fail if all targets failed
		if successCount == 0 {
			return fmt.Errorf("all targets failed during collection")
		}
		
		// Return a warning error that indicates partial success
		return fmt.Errorf("collection completed with %d errors out of %d targets", 
			len(collectionErrors), len(targets))
	}

	return nil
}

// CollectMetricsWithHeaders extends CollectMetrics to support custom headers with error handling
func CollectMetricsWithHeaders(targets []ParsedTargetConfig, storage DataStorage) error {
	// Use the enhanced collection function with error handling
	return CollectMetricsWithRecovery(targets, storage, nil, nil)
}

// collectSingleTargetWithConfig collects data for a single target with full configuration support
func collectSingleTargetWithConfig(target ParsedTargetConfig) (*MonitoringData, error) {
	data := &MonitoringData{
		Timestamp: time.Now(),
		Target:    target.URL,
		Status:    "unknown",
	}

	// Handle TCP vs HTTP/HTTPS
	if strings.HasPrefix(target.URL, "http://") || strings.HasPrefix(target.URL, "https://") {
		return collectHTTPWithConfig(target, data)
	} else {
		return collectTCPWithConfig(target, data)
	}
}

// collectHTTPWithConfig collects HTTP data with full configuration support
func collectHTTPWithConfig(target ParsedTargetConfig, data *MonitoringData) (*MonitoringData, error) {
	var start, dnsStart, connectStart, tlsStart time.Time
	var dnsTime, connectTime, tlsTime, firstByteTime time.Duration
	var sslInfo *SSLData

	// Create HTTP client with custom timeout
	client := &http.Client{
		Timeout: target.Timeout,
	}

	// Create HTTP request
	req, err := http.NewRequest("GET", target.URL, nil)
	if err != nil {
		data.Status = "error"
		data.Error = fmt.Sprintf("Failed to create request: %v", err)
		return data, nil
	}

	// Add custom headers
	for key, value := range target.Headers {
		req.Header.Set(key, value)
	}

	// Setup HTTP tracing
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
		TLSHandshakeDone: func(cs tls.ConnectionState, _ error) {
			tlsTime = time.Since(tlsStart)
			
			// Collect SSL certificate information if SSL check is enabled
			if target.SSLCheck && len(cs.PeerCertificates) > 0 {
				cert := cs.PeerCertificates[0]
				sslInfo = &SSLData{
					ExpiryDate:   cert.NotAfter,
					DaysToExpiry: int(time.Until(cert.NotAfter).Hours() / 24),
					Issuer:       cert.Issuer.CommonName,
					IsValid:      time.Now().Before(cert.NotAfter),
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

	// Handle response
	if err != nil {
		data.Status = "failed"
		data.Error = err.Error()
		data.ResponseTime = totalTime
	} else {
		defer resp.Body.Close()
		data.Status = "success"
		data.StatusCode = resp.StatusCode
		data.ResponseTime = totalTime
		
		// Check against expected status code
		if target.ExpectedStatus != 0 && resp.StatusCode != target.ExpectedStatus {
			data.Status = "failed"
			data.Error = fmt.Sprintf("Expected status %d, got %d", target.ExpectedStatus, resp.StatusCode)
		} else if resp.StatusCode >= 400 {
			data.Status = "failed"
			data.Error = fmt.Sprintf("HTTP %d", resp.StatusCode)
		}
	}

	// Add trace data
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

// collectTCPWithConfig collects TCP data with configuration support
func collectTCPWithConfig(target ParsedTargetConfig, data *MonitoringData) (*MonitoringData, error) {
	start := time.Now()
	
	// Create context with custom timeout
	ctx, cancel := context.WithTimeout(context.Background(), target.Timeout)
	defer cancel()

	// Attempt TCP connection
	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, "tcp", target.URL)
	totalTime := time.Since(start)

	data.ResponseTime = totalTime

	if err != nil {
		data.Status = "failed"
		data.Error = err.Error()
	} else {
		defer conn.Close()
		data.Status = "success"
	}

	return data, nil
}