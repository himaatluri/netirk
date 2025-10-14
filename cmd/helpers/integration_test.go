package helpers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestDataCollectionIntegration tests the complete data collection workflow
func TestDataCollectionIntegration(t *testing.T) {
	// Create test HTTP server with various endpoints
	mux := http.NewServeMux()
	
	// Normal endpoint
	mux.HandleFunc("/ok", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	
	// Slow endpoint
	mux.HandleFunc("/slow", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Slow OK"))
	})
	
	// Error endpoint
	mux.HandleFunc("/error", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	})
	
	// Timeout endpoint
	mux.HandleFunc("/timeout", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second) // Will timeout with 1s timeout
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Too Late"))
	})
	
	server := httptest.NewServer(mux)
	defer server.Close()
	
	// Create target configurations
	targets := []ParsedTargetConfig{
		{
			URL:            server.URL + "/ok",
			Timeout:        1 * time.Second,
			ExpectedStatus: 200,
			Headers:        map[string]string{"User-Agent": "Integration-Test"},
			SSLCheck:       false,
		},
		{
			URL:            server.URL + "/slow",
			Timeout:        1 * time.Second,
			ExpectedStatus: 200,
			Headers:        make(map[string]string),
			SSLCheck:       false,
		},
		{
			URL:            server.URL + "/error",
			Timeout:        1 * time.Second,
			ExpectedStatus: 200, // Expecting 200 but will get 500
			Headers:        make(map[string]string),
			SSLCheck:       false,
		},
		{
			URL:            server.URL + "/timeout",
			Timeout:        500 * time.Millisecond, // Short timeout
			ExpectedStatus: 200,
			Headers:        make(map[string]string),
			SSLCheck:       false,
		},
	}
	
	// Create storage
	config := MonitoringConfig{
		Targets:  []string{server.URL + "/ok", server.URL + "/slow", server.URL + "/error", server.URL + "/timeout"},
		Interval: 30 * time.Second,
	}
	storage := NewInMemoryStorage(config)
	
	// Collect metrics
	err := CollectMetrics(targets, storage)
	if err != nil {
		t.Fatalf("Failed to collect metrics: %v", err)
	}
	
	// Verify data collection
	session := storage.GetSession()
	if len(session.Data) != 4 {
		t.Errorf("Expected 4 data points, got %d", len(session.Data))
	}
	
	// Analyze results by endpoint
	results := make(map[string]*MonitoringData)
	for i := range session.Data {
		data := &session.Data[i]
		if strings.Contains(data.Target, "/ok") {
			results["ok"] = data
		} else if strings.Contains(data.Target, "/slow") {
			results["slow"] = data
		} else if strings.Contains(data.Target, "/error") {
			results["error"] = data
		} else if strings.Contains(data.Target, "/timeout") {
			results["timeout"] = data
		}
	}
	
	// Validate OK endpoint
	if okData := results["ok"]; okData != nil {
		if okData.Status != "success" {
			t.Errorf("Expected OK endpoint to succeed, got status: %s", okData.Status)
		}
		if okData.StatusCode != 200 {
			t.Errorf("Expected status code 200 for OK endpoint, got: %d", okData.StatusCode)
		}
		if okData.ResponseTime <= 0 {
			t.Error("Expected positive response time for OK endpoint")
		}
		if okData.TraceData == nil {
			t.Error("Expected trace data for OK endpoint")
		}
	} else {
		t.Error("Missing data for OK endpoint")
	}
	
	// Validate slow endpoint
	if slowData := results["slow"]; slowData != nil {
		if slowData.Status != "success" {
			t.Errorf("Expected slow endpoint to succeed, got status: %s", slowData.Status)
		}
		if slowData.ResponseTime < 150*time.Millisecond {
			t.Errorf("Expected slow response time, got: %v", slowData.ResponseTime)
		}
	} else {
		t.Error("Missing data for slow endpoint")
	}
	
	// Validate error endpoint
	if errorData := results["error"]; errorData != nil {
		if errorData.Status != "failed" {
			t.Errorf("Expected error endpoint to fail, got status: %s", errorData.Status)
		}
		if errorData.StatusCode != 500 {
			t.Errorf("Expected status code 500 for error endpoint, got: %d", errorData.StatusCode)
		}
		if !strings.Contains(errorData.Error, "Expected status 200, got 500") {
			t.Errorf("Expected status mismatch error, got: %s", errorData.Error)
		}
	} else {
		t.Error("Missing data for error endpoint")
	}
	
	// Validate timeout endpoint
	if timeoutData := results["timeout"]; timeoutData != nil {
		if timeoutData.Status != "failed" {
			t.Errorf("Expected timeout endpoint to fail, got status: %s", timeoutData.Status)
		}
		if !strings.Contains(timeoutData.Error, "timeout") && !strings.Contains(timeoutData.Error, "deadline") {
			t.Errorf("Expected timeout error, got: %s", timeoutData.Error)
		}
	} else {
		t.Error("Missing data for timeout endpoint")
	}
}

// TestAlertingIntegration tests the complete alerting workflow
func TestAlertingIntegration(t *testing.T) {
	// Create webhook test server
	var receivedAlerts []WebhookPayload
	var alertMutex sync.Mutex
	
	webhookServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload WebhookPayload
		err := json.NewDecoder(r.Body).Decode(&payload)
		if err != nil {
			t.Errorf("Failed to decode webhook payload: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		
		alertMutex.Lock()
		receivedAlerts = append(receivedAlerts, payload)
		alertMutex.Unlock()
		
		w.WriteHeader(http.StatusOK)
	}))
	defer webhookServer.Close()
	
	// Create failing test server
	failingServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("Service Unavailable"))
	}))
	defer failingServer.Close()
	
	// Create alert configuration
	alertConfig := AlertConfig{
		Enabled:          true,
		FailureThreshold: 2,
		WebhookURL:       webhookServer.URL,
		SSLWarningDays:   30,
		SSLCriticalDays:  7,
	}
	
	alertManager := NewAlertManager(alertConfig)
	
	// Test complete failure -> alert -> recovery workflow
	t.Run("complete_alert_workflow", func(t *testing.T) {
		target := failingServer.URL
		
		// Clear previous alerts
		alertMutex.Lock()
		receivedAlerts = []WebhookPayload{}
		alertMutex.Unlock()
		
		// First failure - no alert yet
		alert1 := alertManager.ProcessFailure(target, "service unavailable")
		if alert1 != nil {
			t.Error("Expected no alert for first failure")
		}
		
		// Second failure - should trigger alert
		alert2 := alertManager.ProcessFailure(target, "service unavailable")
		if alert2 == nil {
			t.Fatal("Expected alert for second failure")
		}
		
		// Send alert notification
		alertManager.NotifyAlert(alert2)
		
		// Wait for webhook delivery
		time.Sleep(200 * time.Millisecond)
		
		// Verify webhook received failure alert
		alertMutex.Lock()
		if len(receivedAlerts) != 1 {
			t.Errorf("Expected 1 webhook alert, got %d", len(receivedAlerts))
		} else {
			alert := receivedAlerts[0]
			if alert.Alert.Type != AlertTypeFailure {
				t.Errorf("Expected failure alert, got %s", alert.Alert.Type)
			}
			if alert.Alert.Target != target {
				t.Errorf("Expected target %s, got %s", target, alert.Alert.Target)
			}
			if alert.Alert.FailureCount != 2 {
				t.Errorf("Expected failure count 2, got %d", alert.Alert.FailureCount)
			}
		}
		alertMutex.Unlock()
		
		// Process recovery
		recoveryAlert := alertManager.ProcessSuccess(target)
		if recoveryAlert == nil {
			t.Fatal("Expected recovery alert")
		}
		
		// Send recovery notification
		alertManager.NotifyAlert(recoveryAlert)
		
		// Wait for webhook delivery
		time.Sleep(200 * time.Millisecond)
		
		// Verify webhook received recovery alert
		alertMutex.Lock()
		if len(receivedAlerts) != 2 {
			t.Errorf("Expected 2 webhook alerts (failure + recovery), got %d", len(receivedAlerts))
		} else {
			recoveryWebhook := receivedAlerts[1]
			if recoveryWebhook.Alert.Type != AlertTypeRecovery {
				t.Errorf("Expected recovery alert, got %s", recoveryWebhook.Alert.Type)
			}
			if recoveryWebhook.Alert.Target != target {
				t.Errorf("Expected target %s, got %s", target, recoveryWebhook.Alert.Target)
			}
		}
		alertMutex.Unlock()
		
		// Verify alert state is cleared
		if alertManager.IsInAlertState(target) {
			t.Error("Expected alert state to be cleared after recovery")
		}
		if alertManager.GetFailureCount(target) != 0 {
			t.Errorf("Expected failure count to be reset, got %d", alertManager.GetFailureCount(target))
		}
	})
	
	// Test SSL alerts
	t.Run("ssl_alerts", func(t *testing.T) {
		// Clear previous alerts
		alertMutex.Lock()
		receivedAlerts = []WebhookPayload{}
		alertMutex.Unlock()
		
		// Test critical SSL alert
		sslAlert := alertManager.ProcessSSLAlert("secure.example.com", 3, "CN=secure.example.com, Issuer=Let's Encrypt")
		if sslAlert == nil {
			t.Fatal("Expected SSL critical alert for 3 days")
		}
		
		if sslAlert.Severity != SeverityCritical {
			t.Errorf("Expected critical severity for 3-day SSL expiry, got %s", sslAlert.Severity)
		}
		
		// Send SSL notification
		alertManager.NotifyAlert(sslAlert)
		
		// Wait for webhook delivery
		time.Sleep(200 * time.Millisecond)
		
		// Verify SSL webhook
		alertMutex.Lock()
		if len(receivedAlerts) != 1 {
			t.Errorf("Expected 1 SSL webhook alert, got %d", len(receivedAlerts))
		} else {
			alert := receivedAlerts[0]
			if alert.Alert.Type != AlertTypeSSL {
				t.Errorf("Expected SSL alert, got %s", alert.Alert.Type)
			}
			if alert.Alert.Severity != SeverityCritical {
				t.Errorf("Expected critical severity, got %s", alert.Alert.Severity)
			}
		}
		alertMutex.Unlock()
	})
}

// TestDataPersistenceIntegration tests data storage and export workflows
func TestDataPersistenceIntegration(t *testing.T) {
	tempDir := t.TempDir()
	
	// Create test monitoring session with comprehensive data
	config := MonitoringConfig{
		Targets:  []string{"https://example.com", "https://google.com", "tcp://8.8.8.8:53"},
		Interval: 30 * time.Second,
		Duration: 5 * time.Minute,
	}
	
	storage := NewInMemoryStorage(config)
	
	// Add comprehensive test data
	testData := []MonitoringData{
		{
			Timestamp:    time.Now().Add(-4 * time.Minute),
			Target:       "https://example.com",
			Status:       "success",
			ResponseTime: 250 * time.Millisecond,
			StatusCode:   200,
			SSLInfo: &SSLData{
				ExpiryDate:   time.Now().AddDate(0, 3, 0),
				DaysToExpiry: 90,
				Issuer:       "Let's Encrypt",
				IsValid:      true,
			},
			TraceData: &NetworkTrace{
				DNSTime:       15 * time.Millisecond,
				ConnectTime:   45 * time.Millisecond,
				TLSTime:       85 * time.Millisecond,
				FirstByteTime: 250 * time.Millisecond,
			},
		},
		{
			Timestamp:    time.Now().Add(-3 * time.Minute),
			Target:       "https://google.com",
			Status:       "success",
			ResponseTime: 180 * time.Millisecond,
			StatusCode:   200,
			SSLInfo: &SSLData{
				ExpiryDate:   time.Now().AddDate(0, 6, 0),
				DaysToExpiry: 180,
				Issuer:       "Google Trust Services",
				IsValid:      true,
			},
			TraceData: &NetworkTrace{
				DNSTime:       8 * time.Millisecond,
				ConnectTime:   25 * time.Millisecond,
				TLSTime:       65 * time.Millisecond,
				FirstByteTime: 180 * time.Millisecond,
			},
		},
		{
			Timestamp:    time.Now().Add(-2 * time.Minute),
			Target:       "tcp://8.8.8.8:53",
			Status:       "success",
			ResponseTime: 45 * time.Millisecond,
			StatusCode:   0, // TCP doesn't have HTTP status codes
		},
		{
			Timestamp:    time.Now().Add(-1 * time.Minute),
			Target:       "https://example.com",
			Status:       "failed",
			ResponseTime: 0,
			Error:        "connection timeout after 30s",
		},
		{
			Timestamp:    time.Now(),
			Target:       "https://google.com",
			Status:       "success",
			ResponseTime: 195 * time.Millisecond,
			StatusCode:   200,
		},
	}
	
	// Store all test data
	for _, data := range testData {
		err := storage.Store(&data)
		if err != nil {
			t.Fatalf("Failed to store test data: %v", err)
		}
	}
	
	// Test JSON persistence
	t.Run("json_persistence", func(t *testing.T) {
		jsonFile := filepath.Join(tempDir, "test_session.json")
		
		err := storage.SaveToFile(jsonFile)
		if err != nil {
			t.Fatalf("Failed to save JSON file: %v", err)
		}
		
		// Verify file exists and has content
		fileInfo, err := os.Stat(jsonFile)
		if err != nil {
			t.Fatalf("JSON file not created: %v", err)
		}
		if fileInfo.Size() == 0 {
			t.Error("JSON file is empty")
		}
		
		// Load and verify data
		newStorage := NewInMemoryStorage(MonitoringConfig{})
		err = newStorage.LoadFromFile(jsonFile)
		if err != nil {
			t.Fatalf("Failed to load JSON file: %v", err)
		}
		
		loadedSession := newStorage.GetSession()
		if len(loadedSession.Data) != 5 {
			t.Errorf("Expected 5 data points after loading, got %d", len(loadedSession.Data))
		}
		
		// Verify data integrity
		for i, originalData := range testData {
			loadedData := loadedSession.Data[i]
			if loadedData.Target != originalData.Target {
				t.Errorf("Target mismatch at index %d: expected %s, got %s", i, originalData.Target, loadedData.Target)
			}
			if loadedData.Status != originalData.Status {
				t.Errorf("Status mismatch at index %d: expected %s, got %s", i, originalData.Status, loadedData.Status)
			}
		}
	})
	
	// Test export formats
	exportFormats := []struct {
		format    string
		extension string
		validator func(content string) error
	}{
		{
			format:    "csv",
			extension: ".csv",
			validator: func(content string) error {
				lines := strings.Split(content, "\n")
				if len(lines) < 6 { // Header + 5 data rows
					return fmt.Errorf("expected at least 6 lines in CSV, got %d", len(lines))
				}
				if !strings.Contains(lines[0], "timestamp") || !strings.Contains(lines[0], "target") {
					return fmt.Errorf("CSV header missing expected columns")
				}
				return nil
			},
		},
		{
			format:    "html",
			extension: ".html",
			validator: func(content string) error {
				if !strings.Contains(content, "<html>") || !strings.Contains(content, "</html>") {
					return fmt.Errorf("HTML output missing basic structure")
				}
				if !strings.Contains(content, "Monitoring Report") {
					return fmt.Errorf("HTML output missing report title")
				}
				if !strings.Contains(content, "example.com") {
					return fmt.Errorf("HTML output missing target data")
				}
				return nil
			},
		},
	}
	
	for _, format := range exportFormats {
		t.Run(fmt.Sprintf("export_%s", format.format), func(t *testing.T) {
			exportFile := filepath.Join(tempDir, fmt.Sprintf("test_export%s", format.extension))
			
			session := storage.GetSession()
			exporter, err := GetExporter(format.format)
			if err != nil {
				t.Fatalf("Failed to get %s exporter: %v", format.format, err)
			}
			
			err = ExportToFile(session, exporter, exportFile)
			if err != nil {
				t.Fatalf("Failed to export to %s: %v", format.format, err)
			}
			
			// Verify file exists
			if _, err := os.Stat(exportFile); os.IsNotExist(err) {
				t.Fatalf("Export file not created: %s", exportFile)
			}
			
			// Read and validate content
			content, err := os.ReadFile(exportFile)
			if err != nil {
				t.Fatalf("Failed to read export file: %v", err)
			}
			
			if len(content) == 0 {
				t.Error("Export file is empty")
			}
			
			// Format-specific validation
			if err := format.validator(string(content)); err != nil {
				t.Errorf("Export validation failed for %s: %v", format.format, err)
			}
		})
	}
}

// TestConcurrentMonitoringIntegration tests concurrent monitoring scenarios
func TestConcurrentMonitoringIntegration(t *testing.T) {
	// Create multiple test servers with different response patterns
	servers := make([]*httptest.Server, 5)
	
	for i := 0; i < 5; i++ {
		serverIndex := i
		servers[i] = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Different response times and patterns
			switch serverIndex {
			case 0:
				// Fast server
				time.Sleep(50 * time.Millisecond)
				w.WriteHeader(http.StatusOK)
			case 1:
				// Medium server
				time.Sleep(150 * time.Millisecond)
				w.WriteHeader(http.StatusOK)
			case 2:
				// Slow server
				time.Sleep(300 * time.Millisecond)
				w.WriteHeader(http.StatusOK)
			case 3:
				// Intermittent server (50% failure rate)
				if time.Now().UnixNano()%2 == 0 {
					w.WriteHeader(http.StatusOK)
				} else {
					w.WriteHeader(http.StatusServiceUnavailable)
				}
			case 4:
				// Error server
				w.WriteHeader(http.StatusInternalServerError)
			}
		}))
		defer servers[i].Close()
	}
	
	// Create target configurations
	targets := make([]ParsedTargetConfig, len(servers))
	for i, server := range servers {
		targets[i] = ParsedTargetConfig{
			URL:            server.URL,
			Timeout:        1 * time.Second,
			ExpectedStatus: 200,
			Headers:        map[string]string{"X-Server-Index": fmt.Sprintf("%d", i)},
			SSLCheck:       false,
		}
	}
	
	// Create storage
	config := MonitoringConfig{
		Targets:  make([]string, len(servers)),
		Interval: 1 * time.Second,
	}
	for i, server := range servers {
		config.Targets[i] = server.URL
	}
	storage := NewInMemoryStorage(config)
	
	// Perform multiple concurrent collection rounds
	rounds := 3
	for round := 0; round < rounds; round++ {
		t.Run(fmt.Sprintf("round_%d", round+1), func(t *testing.T) {
			startTime := time.Now()
			
			err := CollectMetrics(targets, storage)
			if err != nil {
				t.Fatalf("Failed to collect metrics in round %d: %v", round+1, err)
			}
			
			collectionTime := time.Since(startTime)
			
			// Verify concurrent execution performance
			// Should be much faster than sequential (which would be 50+150+300+variable+0 = 500ms+)
			if collectionTime > 1*time.Second {
				t.Errorf("Round %d took too long (%v), suggesting poor concurrency", round+1, collectionTime)
			}
		})
		
		// Small delay between rounds
		time.Sleep(100 * time.Millisecond)
	}
	
	// Verify final results
	session := storage.GetSession()
	expectedDataPoints := len(servers) * rounds
	if len(session.Data) != expectedDataPoints {
		t.Errorf("Expected %d data points total, got %d", expectedDataPoints, len(session.Data))
	}
	
	// Analyze results by server
	serverResults := make(map[string][]MonitoringData)
	for _, data := range session.Data {
		serverResults[data.Target] = append(serverResults[data.Target], data)
	}
	
	// Verify each server has the expected number of results
	for i, server := range servers {
		results := serverResults[server.URL]
		if len(results) != rounds {
			t.Errorf("Expected %d results for server %d, got %d", rounds, i, len(results))
		}
		
		// Verify server-specific behavior
		switch i {
		case 0, 1, 2: // Fast, medium, slow servers - should mostly succeed
			successCount := 0
			for _, result := range results {
				if result.Status == "success" {
					successCount++
				}
			}
			if successCount < rounds/2 {
				t.Errorf("Expected mostly successful results for server %d, got %d/%d", i, successCount, rounds)
			}
			
		case 4: // Error server - should always fail
			for _, result := range results {
				if result.Status != "failed" {
					t.Errorf("Expected error server to always fail, got status: %s", result.Status)
				}
				if result.StatusCode != 500 {
					t.Errorf("Expected status code 500 for error server, got: %d", result.StatusCode)
				}
			}
		}
	}
}

// TestErrorRecoveryIntegration tests error handling and recovery mechanisms
func TestErrorRecoveryIntegration(t *testing.T) {
	// Test storage error recovery
	t.Run("storage_error_recovery", func(t *testing.T) {
		_ = NewInMemoryStorage(MonitoringConfig{}) // Create storage for potential future use
		
		// Test recovery from storage errors
		errorHandler := NewErrorHandler(nil)
		recoveryManager := NewRecoveryManager(errorHandler, nil)
		
		// Simulate storage operation that might fail
		err := recoveryManager.RecoverStorageOperation(func() error {
			// Simulate a transient error on first attempt
			if recoveryManager.GetRecoveryStats()["storage_retry_count"] == 0 {
				return CreateStorageError("test", fmt.Errorf("transient storage error"))
			}
			return nil
		}, "test_operation")
		
		if err != nil {
			t.Errorf("Expected recovery to succeed, got error: %v", err)
		}
		
		stats := recoveryManager.GetRecoveryStats()
		if stats["storage_retry_count"] == 0 {
			t.Error("Expected retry to be attempted")
		}
	})
	
	// Test network error handling
	t.Run("network_error_handling", func(t *testing.T) {
		// Create a server that will be closed to simulate network errors
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		serverURL := server.URL
		server.Close() // Close immediately to cause connection errors
		
		target := ParsedTargetConfig{
			URL:            serverURL,
			Timeout:        1 * time.Second,
			ExpectedStatus: 200,
			Headers:        make(map[string]string),
		}
		
		// Collect data from closed server
		data, err := collectSingleTargetWithConfig(target)
		if err != nil {
			t.Fatalf("Expected no error from collection function, got: %v", err)
		}
		
		// Should have failure data with error details
		if data.Status != "failed" {
			t.Errorf("Expected failed status for closed server, got: %s", data.Status)
		}
		if data.Error == "" {
			t.Error("Expected error message for failed connection")
		}
		if !strings.Contains(data.Error, "connection") && !strings.Contains(data.Error, "refused") {
			t.Errorf("Expected connection error, got: %s", data.Error)
		}
	})
	
	// Test timeout handling
	t.Run("timeout_handling", func(t *testing.T) {
		// Create server with long response time
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(2 * time.Second) // Longer than timeout
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()
		
		target := ParsedTargetConfig{
			URL:            server.URL,
			Timeout:        500 * time.Millisecond, // Short timeout
			ExpectedStatus: 200,
			Headers:        make(map[string]string),
		}
		
		startTime := time.Now()
		data, err := collectSingleTargetWithConfig(target)
		duration := time.Since(startTime)
		
		if err != nil {
			t.Fatalf("Expected no error from collection function, got: %v", err)
		}
		
		// Should timeout quickly
		if duration > 1*time.Second {
			t.Errorf("Expected timeout within 1s, took: %v", duration)
		}
		
		// Should have failure data
		if data.Status != "failed" {
			t.Errorf("Expected failed status for timeout, got: %s", data.Status)
		}
		if !strings.Contains(data.Error, "timeout") && !strings.Contains(data.Error, "deadline") {
			t.Errorf("Expected timeout error, got: %s", data.Error)
		}
	})
}