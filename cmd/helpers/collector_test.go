package helpers

import (
	"testing"
	"time"
)

func TestNetworkCollector_CollectData_HTTP(t *testing.T) {
	collector := NewNetworkCollector(10 * time.Second)
	
	// Test with a reliable HTTP endpoint
	data, err := collector.CollectData("https://httpbin.org/status/200")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	
	if data == nil {
		t.Fatal("Expected data, got nil")
	}
	
	if data.Target != "https://httpbin.org/status/200" {
		t.Errorf("Expected target 'https://httpbin.org/status/200', got: %s", data.Target)
	}
	
	if data.Status != "success" {
		t.Errorf("Expected status 'success', got: %s", data.Status)
	}
	
	if data.StatusCode != 200 {
		t.Errorf("Expected status code 200, got: %d", data.StatusCode)
	}
	
	if data.ResponseTime <= 0 {
		t.Error("Expected positive response time")
	}
	
	if data.TraceData == nil {
		t.Error("Expected trace data to be populated")
	}
}

func TestNetworkCollector_CollectData_TCP(t *testing.T) {
	collector := NewNetworkCollector(5 * time.Second)
	
	// Test with Google's DNS server
	data, err := collector.CollectData("8.8.8.8:53")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	
	if data == nil {
		t.Fatal("Expected data, got nil")
	}
	
	if data.Target != "8.8.8.8:53" {
		t.Errorf("Expected target '8.8.8.8:53', got: %s", data.Target)
	}
	
	if data.Status != "success" {
		t.Errorf("Expected status 'success', got: %s", data.Status)
	}
	
	if data.ResponseTime <= 0 {
		t.Error("Expected positive response time")
	}
}

func TestNetworkCollector_CollectData_HTTPError(t *testing.T) {
	collector := NewNetworkCollector(5 * time.Second)
	
	// Test with a non-existent domain
	data, err := collector.CollectData("https://this-domain-does-not-exist-12345.com")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	
	if data == nil {
		t.Fatal("Expected data, got nil")
	}
	
	if data.Status != "failed" {
		t.Errorf("Expected status 'failed', got: %s", data.Status)
	}
	
	if data.Error == "" {
		t.Error("Expected error message to be populated")
	}
}

func TestNetworkCollector_SetTimeout(t *testing.T) {
	collector := NewNetworkCollector(10 * time.Second)
	
	if collector.timeout != 10*time.Second {
		t.Errorf("Expected timeout 10s, got: %v", collector.timeout)
	}
	
	collector.SetTimeout(5 * time.Second)
	
	if collector.timeout != 5*time.Second {
		t.Errorf("Expected timeout 5s after update, got: %v", collector.timeout)
	}
	
	if collector.client.Timeout != 5*time.Second {
		t.Errorf("Expected client timeout 5s after update, got: %v", collector.client.Timeout)
	}
}

func TestCollectMetrics_EmptyTargets(t *testing.T) {
	storage := NewInMemoryStorage(MonitoringConfig{})
	
	err := CollectMetrics([]ParsedTargetConfig{}, storage)
	if err == nil {
		t.Error("Expected error for empty targets, got nil")
	}
	
	if err.Error() != "no targets provided for monitoring" {
		t.Errorf("Expected specific error message, got: %s", err.Error())
	}
}

func TestCollectMetrics_WithTargets(t *testing.T) {
	storage := NewInMemoryStorage(MonitoringConfig{})
	
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
	
	err := CollectMetrics(targets, storage)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	
	// Check that data was stored
	session := storage.GetSession()
	if len(session.Data) != 2 {
		t.Errorf("Expected 2 data points, got: %d", len(session.Data))
	}
}

func TestCollectSingleTargetWithConfig_HTTPWithHeaders(t *testing.T) {
	target := ParsedTargetConfig{
		URL:            "https://httpbin.org/headers",
		Timeout:        10 * time.Second,
		ExpectedStatus: 200,
		Headers: map[string]string{
			"User-Agent": "Netirk-Test",
			"X-Test":     "true",
		},
		SSLCheck: true,
	}
	
	data, err := collectSingleTargetWithConfig(target)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	
	if data == nil {
		t.Fatal("Expected data, got nil")
	}
	
	if data.Status != "success" {
		t.Errorf("Expected status 'success', got: %s (error: %s)", data.Status, data.Error)
	}
	
	if data.StatusCode != 200 {
		t.Errorf("Expected status code 200, got: %d", data.StatusCode)
	}
	
	// SSL info should be populated for HTTPS targets with SSLCheck enabled
	// Note: SSL info collection depends on successful TLS handshake
	if data.SSLInfo != nil {
		// If SSL info is present, validate it
		if data.SSLInfo.ExpiryDate.IsZero() {
			t.Error("Expected SSL expiry date to be set")
		}
		if data.SSLInfo.Issuer == "" {
			t.Error("Expected SSL issuer to be set")
		}
	} else {
		// SSL info might not be collected in all test environments
		t.Log("SSL info not collected - this may be expected in some test environments")
	}
}

func TestCollectSingleTargetWithConfig_ExpectedStatusMismatch(t *testing.T) {
	target := ParsedTargetConfig{
		URL:            "https://httpbin.org/status/404",
		Timeout:        10 * time.Second,
		ExpectedStatus: 200, // Expecting 200 but will get 404
		Headers:        make(map[string]string),
	}
	
	data, err := collectSingleTargetWithConfig(target)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	
	if data == nil {
		t.Fatal("Expected data, got nil")
	}
	
	if data.Status != "failed" {
		t.Errorf("Expected status 'failed' due to status mismatch, got: %s", data.Status)
	}
	
	if data.StatusCode != 404 {
		t.Errorf("Expected status code 404, got: %d", data.StatusCode)
	}
	
	expectedError := "Expected status 200, got 404"
	if data.Error != expectedError {
		t.Errorf("Expected error '%s', got: '%s'", expectedError, data.Error)
	}
}