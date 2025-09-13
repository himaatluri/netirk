package cmd

import (
	"context"
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

	"github.com/himasagaratluri/netirk/cmd/helpers"
)

// TestMonitoringWorkflow_EndToEnd tests the complete monitoring workflow
func TestMonitoringWorkflow_EndToEnd(t *testing.T) {
	// Create temporary directory for test files
	tempDir := t.TempDir()
	
	// Create test targets file
	targetsFile := filepath.Join(tempDir, "test_targets.yaml")
	targetsContent := `targets:
  - url: "https://httpbin.org/status/200"
    timeout: "5s"
    expected_status: 200
    ssl_check: true
  - url: "https://httpbin.org/delay/1"
    timeout: "3s"
    expected_status: 200
    headers:
      User-Agent: "Netirk-Integration-Test"
`
	
	err := os.WriteFile(targetsFile, []byte(targetsContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test targets file: %v", err)
	}
	
	// Create output file path
	outputFile := filepath.Join(tempDir, "monitoring_results.json")
	
	// Create monitoring configuration
	config := &MonitorConfig{
		TargetsFile:    targetsFile,
		Interval:       2 * time.Second,
		Duration:       8 * time.Second, // Run for 8 seconds to get multiple data points
		OutputFile:     outputFile,
		OutputFormat:   "json",
		AlertThreshold: 2,
	}
	
	// Run monitoring in a goroutine
	var monitoringErr error
	done := make(chan bool)
	
	go func() {
		defer func() { done <- true }()
		monitoringErr = runMonitoring(config)
	}()
	
	// Wait for monitoring to complete
	select {
	case <-done:
		// Monitoring completed
	case <-time.After(15 * time.Second):
		t.Fatal("Monitoring test timed out")
	}
	
	// Check for monitoring errors
	if monitoringErr != nil {
		t.Fatalf("Monitoring failed: %v", monitoringErr)
	}
	
	// Verify output file was created
	if _, err := os.Stat(outputFile); os.IsNotExist(err) {
		t.Fatal("Output file was not created")
	}
	
	// Read and validate output file
	data, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}
	
	var session helpers.MonitoringSession
	err = json.Unmarshal(data, &session)
	if err != nil {
		t.Fatalf("Failed to unmarshal monitoring session: %v", err)
	}
	
	// Validate session data
	if len(session.Data) == 0 {
		t.Error("Expected monitoring data to be collected")
	}
	
	if len(session.Config.Targets) != 2 {
		t.Errorf("Expected 2 targets in config, got %d", len(session.Config.Targets))
	}
	
	if session.Config.Interval != 2*time.Second {
		t.Errorf("Expected interval 2s, got %v", session.Config.Interval)
	}
	
	// Verify we have data for both targets
	targetsSeen := make(map[string]bool)
	for _, data := range session.Data {
		targetsSeen[data.Target] = true
		
		// Validate data structure
		if data.Timestamp.IsZero() {
			t.Error("Expected timestamp to be set")
		}
		if data.Target == "" {
			t.Error("Expected target to be set")
		}
		if data.Status == "" {
			t.Error("Expected status to be set")
		}
	}
	
	// Should have data for both targets
	if len(targetsSeen) < 2 {
		t.Errorf("Expected data for 2 targets, got data for %d targets", len(targetsSeen))
	}
	
	// Verify session timing
	if session.StartTime.IsZero() {
		t.Error("Expected start time to be set")
	}
	if session.EndTime.IsZero() {
		t.Error("Expected end time to be set")
	}
	if session.EndTime.Before(session.StartTime) {
		t.Error("End time should be after start time")
	}
}

// TestMonitoringWorkflow_ConcurrentTargets tests concurrent monitoring of multiple targets
func TestMonitoringWorkflow_ConcurrentTargets(t *testing.T) {
	// Create test HTTP servers with different behaviors
	server1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond) // Fast response
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server1.Close()
	
	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(300 * time.Millisecond) // Slower response
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server2.Close()
	
	server3 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError) // Error response
		w.Write([]byte("Internal Server Error"))
	}))
	defer server3.Close()
	
	// Create temporary directory for test files
	tempDir := t.TempDir()
	
	// Create test targets file with different configurations
	targetsFile := filepath.Join(tempDir, "concurrent_targets.yaml")
	targetsContent := fmt.Sprintf(`targets:
  - url: "%s"
    timeout: "2s"
    expected_status: 200
    headers:
      X-Test-Target: "fast"
  - url: "%s"
    timeout: "1s"
    expected_status: 200
    headers:
      X-Test-Target: "slow"
  - url: "%s"
    timeout: "2s"
    expected_status: 200
    headers:
      X-Test-Target: "error"
  - url: "tcp://8.8.8.8:53"
    timeout: "1s"
`, server1.URL, server2.URL, server3.URL)
	
	err := os.WriteFile(targetsFile, []byte(targetsContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test targets file: %v", err)
	}
	
	// Create storage for monitoring data
	monitoringConfig := helpers.MonitoringConfig{
		Targets:  []string{server1.URL, server2.URL, server3.URL, "tcp://8.8.8.8:53"},
		Interval: 1 * time.Second,
	}
	storage := helpers.NewInMemoryStorage(monitoringConfig)
	
	// Parse targets
	targets, err := helpers.ParseEnhancedTargetFile(targetsFile)
	if err != nil {
		t.Fatalf("Failed to parse targets file: %v", err)
	}
	
	// Measure concurrent collection time
	startTime := time.Now()
	
	// Collect metrics concurrently
	err = helpers.CollectMetrics(targets.Targets, storage)
	if err != nil {
		t.Fatalf("Failed to collect metrics: %v", err)
	}
	
	collectionTime := time.Since(startTime)
	
	// Verify concurrent execution (should be faster than sequential)
	// Sequential would take at least 100+300+error+dns = ~500ms+
	// Concurrent should be closer to the slowest individual request (~300ms)
	if collectionTime > 2*time.Second {
		t.Errorf("Collection took too long (%v), suggesting non-concurrent execution", collectionTime)
	}
	
	// Verify data was collected for all targets
	session := storage.GetSession()
	if len(session.Data) != 4 {
		t.Errorf("Expected data for 4 targets, got %d", len(session.Data))
	}
	
	// Verify different response patterns
	var fastResponse, slowResponse, errorResponse, tcpResponse *helpers.MonitoringData
	for i := range session.Data {
		data := &session.Data[i]
		switch {
		case strings.Contains(data.Target, server1.URL):
			fastResponse = data
		case strings.Contains(data.Target, server2.URL):
			slowResponse = data
		case strings.Contains(data.Target, server3.URL):
			errorResponse = data
		case strings.Contains(data.Target, "8.8.8.8:53"):
			tcpResponse = data
		}
	}
	
	// Validate fast response
	if fastResponse == nil {
		t.Error("Expected data for fast server")
	} else {
		if fastResponse.Status != "success" {
			t.Errorf("Expected fast server to succeed, got status: %s", fastResponse.Status)
		}
		if fastResponse.ResponseTime > 500*time.Millisecond {
			t.Errorf("Expected fast response time, got: %v", fastResponse.ResponseTime)
		}
	}
	
	// Validate slow response (might timeout due to 1s timeout vs 300ms response)
	if slowResponse == nil {
		t.Error("Expected data for slow server")
	} else {
		// Could be success or timeout depending on timing
		if slowResponse.Status == "success" && slowResponse.ResponseTime < 200*time.Millisecond {
			t.Errorf("Expected slower response time, got: %v", slowResponse.ResponseTime)
		}
	}
	
	// Validate error response
	if errorResponse == nil {
		t.Error("Expected data for error server")
	} else {
		if errorResponse.Status != "failed" {
			t.Errorf("Expected error server to fail, got status: %s", errorResponse.Status)
		}
		if errorResponse.StatusCode != 500 {
			t.Errorf("Expected status code 500, got: %d", errorResponse.StatusCode)
		}
	}
	
	// Validate TCP response
	if tcpResponse == nil {
		t.Error("Expected data for TCP target")
	} else {
		if tcpResponse.Status != "success" {
			t.Errorf("Expected TCP target to succeed, got status: %s (error: %s)", tcpResponse.Status, tcpResponse.Error)
		}
	}
}

// TestMonitoringWorkflow_DataPersistence tests data storage and retrieval
func TestMonitoringWorkflow_DataPersistence(t *testing.T) {
	tempDir := t.TempDir()
	
	// Test different output formats
	testCases := []struct {
		format    string
		extension string
	}{
		{"json", ".json"},
		{"csv", ".csv"},
		{"html", ".html"},
	}
	
	for _, tc := range testCases {
		t.Run(fmt.Sprintf("format_%s", tc.format), func(t *testing.T) {
			// Create test data
			config := helpers.MonitoringConfig{
				Targets:  []string{"https://httpbin.org/status/200"},
				Interval: 30 * time.Second,
			}
			storage := helpers.NewInMemoryStorage(config)
			
			// Add test monitoring data
			testData := []helpers.MonitoringData{
				{
					Timestamp:    time.Now().Add(-2 * time.Minute),
					Target:       "https://httpbin.org/status/200",
					Status:       "success",
					ResponseTime: 200 * time.Millisecond,
					StatusCode:   200,
					TraceData: &helpers.NetworkTrace{
						DNSTime:       10 * time.Millisecond,
						ConnectTime:   50 * time.Millisecond,
						TLSTime:       80 * time.Millisecond,
						FirstByteTime: 200 * time.Millisecond,
					},
				},
				{
					Timestamp:    time.Now().Add(-1 * time.Minute),
					Target:       "https://httpbin.org/status/200",
					Status:       "success",
					ResponseTime: 180 * time.Millisecond,
					StatusCode:   200,
				},
				{
					Timestamp:    time.Now(),
					Target:       "https://httpbin.org/status/200",
					Status:       "failed",
					ResponseTime: 0,
					Error:        "connection timeout",
				},
			}
			
			for _, data := range testData {
				err := storage.Store(&data)
				if err != nil {
					t.Fatalf("Failed to store data: %v", err)
				}
			}
			
			// Test file persistence
			outputFile := filepath.Join(tempDir, fmt.Sprintf("test_output_%s%s", tc.format, tc.extension))
			
			if tc.format == "json" {
				// Test JSON format (default storage format)
				err := storage.SaveToFile(outputFile)
				if err != nil {
					t.Fatalf("Failed to save to JSON file: %v", err)
				}
			} else {
				// Test other formats using exporter
				session := storage.GetSession()
				err := exportWithFormat(session, outputFile, tc.format)
				if err != nil {
					t.Fatalf("Failed to export to %s format: %v", tc.format, err)
				}
			}
			
			// Verify file was created
			if _, err := os.Stat(outputFile); os.IsNotExist(err) {
				t.Fatalf("Output file was not created: %s", outputFile)
			}
			
			// Verify file content
			fileData, err := os.ReadFile(outputFile)
			if err != nil {
				t.Fatalf("Failed to read output file: %v", err)
			}
			
			if len(fileData) == 0 {
				t.Error("Output file is empty")
			}
			
			// Format-specific validation
			switch tc.format {
			case "json":
				// Validate JSON structure
				var session helpers.MonitoringSession
				err = json.Unmarshal(fileData, &session)
				if err != nil {
					t.Fatalf("Invalid JSON format: %v", err)
				}
				
				if len(session.Data) != 3 {
					t.Errorf("Expected 3 data points in JSON, got %d", len(session.Data))
				}
				
			case "csv":
				// Validate CSV structure
				content := string(fileData)
				lines := strings.Split(content, "\n")
				if len(lines) < 4 { // Header + 3 data rows + possible empty line
					t.Errorf("Expected at least 4 lines in CSV, got %d", len(lines))
				}
				
				// Check for CSV header
				if !strings.Contains(lines[0], "timestamp") || !strings.Contains(lines[0], "target") {
					t.Error("CSV header missing expected columns")
				}
				
			case "html":
				// Validate HTML structure
				content := string(fileData)
				if !strings.Contains(content, "<html>") || !strings.Contains(content, "</html>") {
					t.Error("HTML output missing basic HTML structure")
				}
				if !strings.Contains(content, "Monitoring Report") {
					t.Error("HTML output missing report title")
				}
			}
		})
	}
}

// TestMonitoringWorkflow_AlertDelivery tests alert mechanisms
func TestMonitoringWorkflow_AlertDelivery(t *testing.T) {
	// Create test webhook server
	var webhookPayloads []helpers.WebhookPayload
	var webhookMutex sync.Mutex
	
	webhookServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload helpers.WebhookPayload
		err := json.NewDecoder(r.Body).Decode(&payload)
		if err != nil {
			t.Errorf("Failed to decode webhook payload: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		
		webhookMutex.Lock()
		webhookPayloads = append(webhookPayloads, payload)
		webhookMutex.Unlock()
		
		w.WriteHeader(http.StatusOK)
	}))
	defer webhookServer.Close()
	
	// Create failing test server
	failingServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Server Error"))
	}))
	defer failingServer.Close()
	
	// Create alert manager with webhook configuration
	alertConfig := helpers.AlertConfig{
		Enabled:          true,
		FailureThreshold: 2,
		WebhookURL:       webhookServer.URL,
		SSLWarningDays:   30,
		SSLCriticalDays:  7,
	}
	
	alertManager := helpers.NewAlertManager(alertConfig)
	
	// Test failure alerts
	t.Run("failure_alerts", func(t *testing.T) {
		// First failure - should not trigger alert
		alert1 := alertManager.ProcessFailure(failingServer.URL, "connection error")
		if alert1 != nil {
			t.Error("Expected no alert for first failure")
		}
		
		// Second failure - should trigger alert
		alert2 := alertManager.ProcessFailure(failingServer.URL, "connection error")
		if alert2 == nil {
			t.Fatal("Expected alert for second failure")
		}
		
		// Verify alert properties
		if alert2.Type != helpers.AlertTypeFailure {
			t.Errorf("Expected failure alert type, got %s", alert2.Type)
		}
		if alert2.Severity != helpers.SeverityCritical {
			t.Errorf("Expected critical severity, got %s", alert2.Severity)
		}
		if alert2.FailureCount != 2 {
			t.Errorf("Expected failure count 2, got %d", alert2.FailureCount)
		}
		
		// Send alert notification
		alertManager.NotifyAlert(alert2)
		
		// Wait for webhook delivery
		time.Sleep(100 * time.Millisecond)
		
		// Verify webhook was called
		webhookMutex.Lock()
		if len(webhookPayloads) == 0 {
			t.Error("Expected webhook to be called")
		} else {
			payload := webhookPayloads[0]
			if payload.Alert.Type != helpers.AlertTypeFailure {
				t.Errorf("Expected failure alert in webhook, got %s", payload.Alert.Type)
			}
			if payload.Source != "netirk" {
				t.Errorf("Expected source 'netirk', got %s", payload.Source)
			}
		}
		webhookMutex.Unlock()
	})
	
	// Test recovery alerts
	t.Run("recovery_alerts", func(t *testing.T) {
		// Clear previous webhook payloads
		webhookMutex.Lock()
		webhookPayloads = []helpers.WebhookPayload{}
		webhookMutex.Unlock()
		
		// Process success after failures - should trigger recovery alert
		recoveryAlert := alertManager.ProcessSuccess(failingServer.URL)
		if recoveryAlert == nil {
			t.Fatal("Expected recovery alert")
		}
		
		// Verify recovery alert properties
		if recoveryAlert.Type != helpers.AlertTypeRecovery {
			t.Errorf("Expected recovery alert type, got %s", recoveryAlert.Type)
		}
		if recoveryAlert.Severity != helpers.SeverityWarning {
			t.Errorf("Expected warning severity for recovery, got %s", recoveryAlert.Severity)
		}
		
		// Send recovery notification
		alertManager.NotifyAlert(recoveryAlert)
		
		// Wait for webhook delivery
		time.Sleep(100 * time.Millisecond)
		
		// Verify recovery webhook was called
		webhookMutex.Lock()
		if len(webhookPayloads) == 0 {
			t.Error("Expected recovery webhook to be called")
		} else {
			payload := webhookPayloads[0]
			if payload.Alert.Type != helpers.AlertTypeRecovery {
				t.Errorf("Expected recovery alert in webhook, got %s", payload.Alert.Type)
			}
		}
		webhookMutex.Unlock()
	})
	
	// Test SSL alerts
	t.Run("ssl_alerts", func(t *testing.T) {
		// Clear previous webhook payloads
		webhookMutex.Lock()
		webhookPayloads = []helpers.WebhookPayload{}
		webhookMutex.Unlock()
		
		// Test SSL critical alert (5 days to expiry)
		sslAlert := alertManager.ProcessSSLAlert("secure.example.com", 5, "CN=secure.example.com, Issuer=Let's Encrypt")
		if sslAlert == nil {
			t.Fatal("Expected SSL critical alert")
		}
		
		// Verify SSL alert properties
		if sslAlert.Type != helpers.AlertTypeSSL {
			t.Errorf("Expected SSL alert type, got %s", sslAlert.Type)
		}
		if sslAlert.Severity != helpers.SeverityCritical {
			t.Errorf("Expected critical severity for 5-day SSL expiry, got %s", sslAlert.Severity)
		}
		
		// Send SSL notification
		alertManager.NotifyAlert(sslAlert)
		
		// Wait for webhook delivery
		time.Sleep(100 * time.Millisecond)
		
		// Verify SSL webhook was called
		webhookMutex.Lock()
		if len(webhookPayloads) == 0 {
			t.Error("Expected SSL webhook to be called")
		} else {
			payload := webhookPayloads[0]
			if payload.Alert.Type != helpers.AlertTypeSSL {
				t.Errorf("Expected SSL alert in webhook, got %s", payload.Alert.Type)
			}
		}
		webhookMutex.Unlock()
	})
}

// TestMonitoringWorkflow_ErrorHandling tests error handling and recovery
func TestMonitoringWorkflow_ErrorHandling(t *testing.T) {
	tempDir := t.TempDir()
	
	// Test with invalid targets file
	t.Run("invalid_targets_file", func(t *testing.T) {
		config := &MonitorConfig{
			TargetsFile:    "/nonexistent/file.yaml",
			Interval:       1 * time.Second,
			Duration:       2 * time.Second,
			AlertThreshold: 1,
		}
		
		err := runMonitoring(config)
		if err == nil {
			t.Error("Expected error for nonexistent targets file")
		}
		if !strings.Contains(err.Error(), "failed to parse targets file") {
			t.Errorf("Expected parse error, got: %v", err)
		}
	})
	
	// Test with malformed YAML
	t.Run("malformed_yaml", func(t *testing.T) {
		malformedFile := filepath.Join(tempDir, "malformed.yaml")
		malformedContent := `targets:
  - url: "https://example.com"
    timeout: invalid_duration
    expected_status: not_a_number
`
		err := os.WriteFile(malformedFile, []byte(malformedContent), 0644)
		if err != nil {
			t.Fatalf("Failed to create malformed file: %v", err)
		}
		
		config := &MonitorConfig{
			TargetsFile:    malformedFile,
			Interval:       1 * time.Second,
			Duration:       2 * time.Second,
			AlertThreshold: 1,
		}
		
		err = runMonitoring(config)
		if err == nil {
			t.Error("Expected error for malformed YAML")
		}
	})
	
	// Test with empty targets file
	t.Run("empty_targets", func(t *testing.T) {
		emptyFile := filepath.Join(tempDir, "empty.yaml")
		emptyContent := `targets: []`
		err := os.WriteFile(emptyFile, []byte(emptyContent), 0644)
		if err != nil {
			t.Fatalf("Failed to create empty file: %v", err)
		}
		
		config := &MonitorConfig{
			TargetsFile:    emptyFile,
			Interval:       1 * time.Second,
			Duration:       2 * time.Second,
			AlertThreshold: 1,
		}
		
		err = runMonitoring(config)
		if err == nil {
			t.Error("Expected error for empty targets")
		}
		if !strings.Contains(err.Error(), "no targets found") {
			t.Errorf("Expected 'no targets found' error, got: %v", err)
		}
	})
	
	// Test storage error handling
	t.Run("storage_errors", func(t *testing.T) {
		storage := helpers.NewInMemoryStorage(helpers.MonitoringConfig{})
		
		// Test storing nil data
		err := storage.Store(nil)
		if err == nil {
			t.Error("Expected error when storing nil data")
		}
		
		// Test saving to invalid path
		err = storage.SaveToFile("/invalid/path/file.json")
		if err == nil {
			t.Error("Expected error when saving to invalid path")
		}
		
		// Test loading from nonexistent file
		err = storage.LoadFromFile("/nonexistent/file.json")
		if err == nil {
			t.Error("Expected error when loading from nonexistent file")
		}
	})
}

// TestMonitoringWorkflow_GracefulShutdown tests graceful shutdown behavior
func TestMonitoringWorkflow_GracefulShutdown(t *testing.T) {
	tempDir := t.TempDir()
	
	// Create test targets file
	targetsFile := filepath.Join(tempDir, "shutdown_test.yaml")
	targetsContent := `targets:
  - url: "https://httpbin.org/delay/1"
    timeout: "5s"
    expected_status: 200
`
	err := os.WriteFile(targetsFile, []byte(targetsContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test targets file: %v", err)
	}
	
	outputFile := filepath.Join(tempDir, "shutdown_test.json")
	
	config := &MonitorConfig{
		TargetsFile:    targetsFile,
		Interval:       1 * time.Second,
		Duration:       0, // Run indefinitely
		OutputFile:     outputFile,
		AlertThreshold: 1,
	}
	
	// Start monitoring in a goroutine
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan bool)
	
	go func() {
		defer func() { done <- true }()
		
		// Set up monitoring context for cancellation
		monitoringCtx = ctx
		monitoringCancel = cancel
		
		_ = runMonitoring(config) // Ignore error for graceful shutdown test
	}()
	
	// Let monitoring run for a few seconds
	time.Sleep(3 * time.Second)
	
	// Cancel monitoring (simulate Ctrl+C)
	cancel()
	
	// Wait for graceful shutdown
	select {
	case <-done:
		// Monitoring completed gracefully
	case <-time.After(5 * time.Second):
		t.Fatal("Monitoring did not shut down gracefully")
	}
	
	// Verify output file was created during shutdown
	if _, err := os.Stat(outputFile); os.IsNotExist(err) {
		t.Error("Output file was not created during graceful shutdown")
	} else {
		// Verify file contains data
		data, err := os.ReadFile(outputFile)
		if err != nil {
			t.Fatalf("Failed to read shutdown output file: %v", err)
		}
		
		if len(data) == 0 {
			t.Error("Shutdown output file is empty")
		}
		
		var session helpers.MonitoringSession
		err = json.Unmarshal(data, &session)
		if err != nil {
			t.Fatalf("Failed to unmarshal shutdown session: %v", err)
		}
		
		if len(session.Data) == 0 {
			t.Error("Expected some monitoring data to be saved during shutdown")
		}
	}
}

// TestMonitoringWorkflow_PerformanceUnderLoad tests monitoring performance with many targets
func TestMonitoringWorkflow_PerformanceUnderLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}
	
	// Create multiple test servers
	servers := make([]*httptest.Server, 10)
	for i := 0; i < 10; i++ {
		servers[i] = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Add small random delay
			time.Sleep(time.Duration(50+i*10) * time.Millisecond)
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		}))
		defer servers[i].Close()
	}
	
	// Create storage
	targets := make([]string, len(servers))
	for i, server := range servers {
		targets[i] = server.URL
	}
	
	config := helpers.MonitoringConfig{
		Targets:  targets,
		Interval: 1 * time.Second,
	}
	storage := helpers.NewInMemoryStorage(config)
	
	// Create target configurations
	targetConfigs := make([]helpers.ParsedTargetConfig, len(servers))
	for i, server := range servers {
		targetConfigs[i] = helpers.ParsedTargetConfig{
			URL:            server.URL,
			Timeout:        2 * time.Second,
			ExpectedStatus: 200,
			Headers:        make(map[string]string),
		}
	}
	
	// Measure collection performance
	startTime := time.Now()
	
	err := helpers.CollectMetrics(targetConfigs, storage)
	if err != nil {
		t.Fatalf("Failed to collect metrics under load: %v", err)
	}
	
	collectionTime := time.Since(startTime)
	
	// Verify all targets were processed
	session := storage.GetSession()
	if len(session.Data) != 10 {
		t.Errorf("Expected data for 10 targets, got %d", len(session.Data))
	}
	
	// Performance should be reasonable (concurrent execution)
	// Sequential would take at least 50+60+70+...+140 = 950ms
	// Concurrent should be closer to the slowest individual request (~140ms)
	if collectionTime > 1*time.Second {
		t.Errorf("Collection took too long (%v) for 10 targets, suggesting poor concurrency", collectionTime)
	}
	
	// Verify all targets have data
	targetsSeen := make(map[string]bool)
	for _, data := range session.Data {
		targetsSeen[data.Target] = true
		
		if data.Status != "success" {
			t.Errorf("Expected success for target %s, got %s", data.Target, data.Status)
		}
		if data.ResponseTime <= 0 {
			t.Errorf("Expected positive response time for target %s", data.Target)
		}
	}
	
	if len(targetsSeen) != 10 {
		t.Errorf("Expected data for 10 unique targets, got %d", len(targetsSeen))
	}
}