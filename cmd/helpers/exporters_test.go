package helpers

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

func TestJSONExporter_Export(t *testing.T) {
	// Create test monitoring session
	session := createTestMonitoringSession()
	
	// Test JSON export
	exporter := NewJSONExporter()
	var buf bytes.Buffer
	
	err := exporter.Export(session, &buf)
	if err != nil {
		t.Fatalf("JSON export failed: %v", err)
	}
	
	// Verify JSON structure
	var result map[string]interface{}
	err = json.Unmarshal(buf.Bytes(), &result)
	if err != nil {
		t.Fatalf("Failed to parse exported JSON: %v", err)
	}
	
	// Check top-level structure
	monitoringSession, ok := result["monitoring_session"].(map[string]interface{})
	if !ok {
		t.Fatal("Missing monitoring_session in JSON output")
	}
	
	// Check required fields
	if _, ok := monitoringSession["start_time"]; !ok {
		t.Error("Missing start_time in JSON output")
	}
	if _, ok := monitoringSession["config"]; !ok {
		t.Error("Missing config in JSON output")
	}
	if _, ok := monitoringSession["data"]; !ok {
		t.Error("Missing data in JSON output")
	}
	
	// Check data array
	data, ok := monitoringSession["data"].([]interface{})
	if !ok {
		t.Fatal("Data field is not an array")
	}
	
	if len(data) != 2 {
		t.Errorf("Expected 2 data points, got %d", len(data))
	}
	
	// Check first data point structure
	firstPoint, ok := data[0].(map[string]interface{})
	if !ok {
		t.Fatal("First data point is not an object")
	}
	
	expectedFields := []string{"timestamp", "target", "status", "response_time_ms"}
	for _, field := range expectedFields {
		if _, ok := firstPoint[field]; !ok {
			t.Errorf("Missing field %s in data point", field)
		}
	}
}

func TestJSONExporter_GetContentType(t *testing.T) {
	exporter := NewJSONExporter()
	contentType := exporter.GetContentType()
	
	if contentType != "application/json" {
		t.Errorf("Expected content type 'application/json', got '%s'", contentType)
	}
}

func TestJSONExporter_GetFileExtension(t *testing.T) {
	exporter := NewJSONExporter()
	extension := exporter.GetFileExtension()
	
	if extension != ".json" {
		t.Errorf("Expected file extension '.json', got '%s'", extension)
	}
}

func TestCSVExporter_Export(t *testing.T) {
	// Create test monitoring session
	session := createTestMonitoringSession()
	
	// Test CSV export
	exporter := NewCSVExporter()
	var buf bytes.Buffer
	
	err := exporter.Export(session, &buf)
	if err != nil {
		t.Fatalf("CSV export failed: %v", err)
	}
	
	// Parse CSV output
	reader := csv.NewReader(strings.NewReader(buf.String()))
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("Failed to parse exported CSV: %v", err)
	}
	
	// Check header row
	if len(records) < 1 {
		t.Fatal("CSV output is empty")
	}
	
	header := records[0]
	expectedHeaders := []string{
		"timestamp", "target", "status", "response_time_ms", "status_code", "error",
		"dns_time_ms", "connect_time_ms", "tls_time_ms", "first_byte_time_ms",
		"ssl_expiry_date", "ssl_days_to_expiry", "ssl_issuer", "ssl_is_valid", "ssl_error",
	}
	
	if len(header) != len(expectedHeaders) {
		t.Errorf("Expected %d headers, got %d", len(expectedHeaders), len(header))
	}
	
	for i, expected := range expectedHeaders {
		if i < len(header) && header[i] != expected {
			t.Errorf("Expected header[%d] to be '%s', got '%s'", i, expected, header[i])
		}
	}
	
	// Check data rows (should be 3 total: 1 header + 2 data)
	if len(records) != 3 {
		t.Errorf("Expected 3 CSV records (1 header + 2 data), got %d", len(records))
	}
	
	// Check first data row
	if len(records) > 1 {
		firstRow := records[1]
		if len(firstRow) != len(expectedHeaders) {
			t.Errorf("Expected %d columns in data row, got %d", len(expectedHeaders), len(firstRow))
		}
		
		// Check that timestamp is not empty
		if firstRow[0] == "" {
			t.Error("Timestamp should not be empty")
		}
		
		// Check that target is correct
		if firstRow[1] != "https://example.com" {
			t.Errorf("Expected target 'https://example.com', got '%s'", firstRow[1])
		}
		
		// Check that status is correct
		if firstRow[2] != "success" {
			t.Errorf("Expected status 'success', got '%s'", firstRow[2])
		}
	}
}

func TestCSVExporter_GetContentType(t *testing.T) {
	exporter := NewCSVExporter()
	contentType := exporter.GetContentType()
	
	if contentType != "text/csv" {
		t.Errorf("Expected content type 'text/csv', got '%s'", contentType)
	}
}

func TestCSVExporter_GetFileExtension(t *testing.T) {
	exporter := NewCSVExporter()
	extension := exporter.GetFileExtension()
	
	if extension != ".csv" {
		t.Errorf("Expected file extension '.csv', got '%s'", extension)
	}
}

func TestExportToFile(t *testing.T) {
	session := createTestMonitoringSession()
	exporter := NewJSONExporter()
	
	// Create temporary file
	tmpFile := "test_export.json"
	defer os.Remove(tmpFile)
	
	// Test export to file
	err := ExportToFile(session, exporter, tmpFile)
	if err != nil {
		t.Fatalf("ExportToFile failed: %v", err)
	}
	
	// Verify file was created and contains data
	data, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("Failed to read exported file: %v", err)
	}
	
	if len(data) == 0 {
		t.Error("Exported file is empty")
	}
	
	// Verify it's valid JSON
	var result map[string]interface{}
	err = json.Unmarshal(data, &result)
	if err != nil {
		t.Fatalf("Exported file does not contain valid JSON: %v", err)
	}
}

func TestExportToFile_ErrorCases(t *testing.T) {
	session := createTestMonitoringSession()
	exporter := NewJSONExporter()
	
	// Test nil session
	err := ExportToFile(nil, exporter, "test.json")
	if err == nil {
		t.Error("Expected error for nil session")
	}
	
	// Test nil exporter
	err = ExportToFile(session, nil, "test.json")
	if err == nil {
		t.Error("Expected error for nil exporter")
	}
	
	// Test empty filename
	err = ExportToFile(session, exporter, "")
	if err == nil {
		t.Error("Expected error for empty filename")
	}
	
	// Test invalid file path
	err = ExportToFile(session, exporter, "/invalid/path/test.json")
	if err == nil {
		t.Error("Expected error for invalid file path")
	}
}

func TestHTMLExporter_Export(t *testing.T) {
	// Create test monitoring session
	session := createTestMonitoringSession()
	
	// Test HTML export
	exporter := NewHTMLExporter()
	var buf bytes.Buffer
	
	err := exporter.Export(session, &buf)
	if err != nil {
		t.Fatalf("HTML export failed: %v", err)
	}
	
	html := buf.String()
	
	// Verify HTML structure
	if !strings.Contains(html, "<!DOCTYPE html>") {
		t.Error("HTML output missing DOCTYPE declaration")
	}
	
	if !strings.Contains(html, "<title>Netirk Monitoring Report</title>") {
		t.Error("HTML output missing title")
	}
	
	// Check for essential sections
	if !strings.Contains(html, "Netirk Monitoring Report") {
		t.Error("HTML output missing main heading")
	}
	
	if !strings.Contains(html, "Total Checks") {
		t.Error("HTML output missing summary section")
	}
	
	if !strings.Contains(html, "Response Time Trends") {
		t.Error("HTML output missing response time chart")
	}
	
	if !strings.Contains(html, "Success Rate by Target") {
		t.Error("HTML output missing success rate chart")
	}
	
	if !strings.Contains(html, "Detailed Monitoring Data") {
		t.Error("HTML output missing data table")
	}
	
	// Check for Chart.js inclusion
	if !strings.Contains(html, "chart.js") {
		t.Error("HTML output missing Chart.js library")
	}
	
	// Check for data in table
	if !strings.Contains(html, "https://example.com") {
		t.Error("HTML output missing target data")
	}
	
	if !strings.Contains(html, "success") {
		t.Error("HTML output missing status data")
	}
	
	// Check for CSS styling
	if !strings.Contains(html, "<style>") {
		t.Error("HTML output missing CSS styles")
	}
	
	// Check for JavaScript charts
	if !strings.Contains(html, "responseTimeChart") {
		t.Error("HTML output missing response time chart JavaScript")
	}
	
	if !strings.Contains(html, "successRateChart") {
		t.Error("HTML output missing success rate chart JavaScript")
	}
}

func TestHTMLExporter_ExportEmptySession(t *testing.T) {
	// Create empty monitoring session
	session := &MonitoringSession{
		StartTime: time.Now().Add(-1 * time.Hour),
		EndTime:   time.Now(),
		Config: MonitoringConfig{
			Targets:  []string{},
			Interval: 30 * time.Second,
		},
		Data: []MonitoringData{},
	}
	
	// Test HTML export with empty data
	exporter := NewHTMLExporter()
	var buf bytes.Buffer
	
	err := exporter.Export(session, &buf)
	if err != nil {
		t.Fatalf("HTML export failed for empty session: %v", err)
	}
	
	html := buf.String()
	
	// Should still contain basic structure
	if !strings.Contains(html, "<!DOCTYPE html>") {
		t.Error("HTML output missing DOCTYPE declaration")
	}
	
	if !strings.Contains(html, "Total Checks") {
		t.Error("HTML output missing summary section")
	}
	
	// Should handle empty data gracefully
	if !strings.Contains(html, "No monitoring data available") {
		t.Error("HTML output should indicate no data available")
	}
}

func TestHTMLExporter_GetContentType(t *testing.T) {
	exporter := NewHTMLExporter()
	contentType := exporter.GetContentType()
	
	if contentType != "text/html" {
		t.Errorf("Expected content type 'text/html', got '%s'", contentType)
	}
}

func TestHTMLExporter_GetFileExtension(t *testing.T) {
	exporter := NewHTMLExporter()
	extension := exporter.GetFileExtension()
	
	if extension != ".html" {
		t.Errorf("Expected file extension '.html', got '%s'", extension)
	}
}

func TestHTMLExporter_GenerateSummary(t *testing.T) {
	session := createTestMonitoringSession()
	exporter := NewHTMLExporter()
	
	summary := exporter.generateSummary(session)
	
	// Check for summary cards
	if !strings.Contains(summary, "Total Checks") {
		t.Error("Summary missing total checks")
	}
	
	if !strings.Contains(summary, "Success Rate") {
		t.Error("Summary missing success rate")
	}
	
	if !strings.Contains(summary, "Targets Monitored") {
		t.Error("Summary missing targets monitored")
	}
	
	if !strings.Contains(summary, "Average Response Time") {
		t.Error("Summary missing average response time")
	}
	
	// Check for correct values
	if !strings.Contains(summary, ">2<") { // 2 total checks
		t.Error("Summary should show 2 total checks")
	}
	
	if !strings.Contains(summary, "50.0") { // 50% success rate (1 success out of 2)
		t.Error("Summary should show 50% success rate")
	}
}

func TestHTMLExporter_GenerateDataTable(t *testing.T) {
	session := createTestMonitoringSession()
	exporter := NewHTMLExporter()
	
	table := exporter.generateDataTable(session)
	
	// Check for table structure
	if !strings.Contains(table, "<table>") {
		t.Error("Data table missing table element")
	}
	
	if !strings.Contains(table, "<thead>") {
		t.Error("Data table missing table header")
	}
	
	if !strings.Contains(table, "<tbody>") {
		t.Error("Data table missing table body")
	}
	
	// Check for headers
	expectedHeaders := []string{"Time", "Target", "Status", "Response Time", "Status Code", "SSL Expiry", "Error"}
	for _, header := range expectedHeaders {
		if !strings.Contains(table, header) {
			t.Errorf("Data table missing header: %s", header)
		}
	}
	
	// Check for data
	if !strings.Contains(table, "https://example.com") {
		t.Error("Data table missing target data")
	}
	
	if !strings.Contains(table, "status-success") {
		t.Error("Data table missing success status class")
	}
	
	if !strings.Contains(table, "status-failure") {
		t.Error("Data table missing failure status class")
	}
}

func TestHTMLExporter_FormatDuration(t *testing.T) {
	exporter := NewHTMLExporter()
	
	// Test seconds
	duration := 45 * time.Second
	formatted := exporter.formatDuration(duration)
	if formatted != "45s" {
		t.Errorf("Expected '45s', got '%s'", formatted)
	}
	
	// Test minutes
	duration = 2*time.Minute + 30*time.Second
	formatted = exporter.formatDuration(duration)
	if formatted != "2.5m" {
		t.Errorf("Expected '2.5m', got '%s'", formatted)
	}
	
	// Test hours
	duration = 2*time.Hour + 30*time.Minute
	formatted = exporter.formatDuration(duration)
	if formatted != "2h 30m" {
		t.Errorf("Expected '2h 30m', got '%s'", formatted)
	}
}

func TestGetExporter(t *testing.T) {
	// Test valid formats
	jsonExporter, err := GetExporter("json")
	if err != nil {
		t.Errorf("Failed to get JSON exporter: %v", err)
	}
	if jsonExporter == nil {
		t.Error("JSON exporter is nil")
	}
	
	csvExporter, err := GetExporter("csv")
	if err != nil {
		t.Errorf("Failed to get CSV exporter: %v", err)
	}
	if csvExporter == nil {
		t.Error("CSV exporter is nil")
	}
	
	htmlExporter, err := GetExporter("html")
	if err != nil {
		t.Errorf("Failed to get HTML exporter: %v", err)
	}
	if htmlExporter == nil {
		t.Error("HTML exporter is nil")
	}
	
	// Test case insensitive
	jsonExporter2, err := GetExporter("JSON")
	if err != nil {
		t.Errorf("Failed to get JSON exporter with uppercase: %v", err)
	}
	if jsonExporter2 == nil {
		t.Error("JSON exporter with uppercase is nil")
	}
	
	htmlExporter2, err := GetExporter("HTML")
	if err != nil {
		t.Errorf("Failed to get HTML exporter with uppercase: %v", err)
	}
	if htmlExporter2 == nil {
		t.Error("HTML exporter with uppercase is nil")
	}
	
	// Test invalid format
	_, err = GetExporter("invalid")
	if err == nil {
		t.Error("Expected error for invalid format")
	}
}

func TestConvertToExportFormat(t *testing.T) {
	// Create test monitoring data
	timestamp := time.Now()
	data := MonitoringData{
		Timestamp:    timestamp,
		Target:       "https://example.com",
		Status:       "success",
		ResponseTime: 250 * time.Millisecond,
		StatusCode:   200,
		Error:        "",
		TraceData: &NetworkTrace{
			DNSTime:       10 * time.Millisecond,
			ConnectTime:   50 * time.Millisecond,
			TLSTime:       100 * time.Millisecond,
			FirstByteTime: 200 * time.Millisecond,
		},
		SSLInfo: &SSLData{
			ExpiryDate:   timestamp.Add(30 * 24 * time.Hour),
			DaysToExpiry: 30,
			Issuer:       "Let's Encrypt",
			IsValid:      true,
			Error:        "",
		},
	}
	
	// Convert to export format
	exported := convertToExportFormat(data)
	
	// Verify conversion
	if exported.Timestamp != timestamp.Format(time.RFC3339) {
		t.Errorf("Expected timestamp '%s', got '%s'", timestamp.Format(time.RFC3339), exported.Timestamp)
	}
	
	if exported.Target != "https://example.com" {
		t.Errorf("Expected target 'https://example.com', got '%s'", exported.Target)
	}
	
	if exported.Status != "success" {
		t.Errorf("Expected status 'success', got '%s'", exported.Status)
	}
	
	if exported.ResponseTimeMs != 250 {
		t.Errorf("Expected response time 250ms, got %d", exported.ResponseTimeMs)
	}
	
	if exported.StatusCode != 200 {
		t.Errorf("Expected status code 200, got %d", exported.StatusCode)
	}
	
	// Check trace data conversion
	if exported.TraceData == nil {
		t.Fatal("Trace data is nil")
	}
	
	if exported.TraceData.DNSTimeMs != 10 {
		t.Errorf("Expected DNS time 10ms, got %d", exported.TraceData.DNSTimeMs)
	}
	
	if exported.TraceData.ConnectTimeMs != 50 {
		t.Errorf("Expected connect time 50ms, got %d", exported.TraceData.ConnectTimeMs)
	}
	
	// Check SSL data conversion
	if exported.SSLInfo == nil {
		t.Fatal("SSL info is nil")
	}
	
	if exported.SSLInfo.DaysToExpiry != 30 {
		t.Errorf("Expected days to expiry 30, got %d", exported.SSLInfo.DaysToExpiry)
	}
	
	if exported.SSLInfo.Issuer != "Let's Encrypt" {
		t.Errorf("Expected issuer 'Let's Encrypt', got '%s'", exported.SSLInfo.Issuer)
	}
	
	if !exported.SSLInfo.IsValid {
		t.Error("Expected SSL to be valid")
	}
}

// Helper function to create test monitoring session
func createTestMonitoringSession() *MonitoringSession {
	startTime := time.Now().Add(-1 * time.Hour)
	endTime := time.Now()
	
	config := MonitoringConfig{
		Targets:  []string{"https://example.com", "https://test.com"},
		Interval: 30 * time.Second,
		Duration: 1 * time.Hour,
	}
	
	data := []MonitoringData{
		{
			Timestamp:    startTime.Add(10 * time.Minute),
			Target:       "https://example.com",
			Status:       "success",
			ResponseTime: 250 * time.Millisecond,
			StatusCode:   200,
			Error:        "",
			TraceData: &NetworkTrace{
				DNSTime:       10 * time.Millisecond,
				ConnectTime:   50 * time.Millisecond,
				TLSTime:       100 * time.Millisecond,
				FirstByteTime: 200 * time.Millisecond,
			},
			SSLInfo: &SSLData{
				ExpiryDate:   startTime.Add(30 * 24 * time.Hour),
				DaysToExpiry: 30,
				Issuer:       "Let's Encrypt",
				IsValid:      true,
				Error:        "",
			},
		},
		{
			Timestamp:    startTime.Add(20 * time.Minute),
			Target:       "https://test.com",
			Status:       "failure",
			ResponseTime: 0,
			StatusCode:   0,
			Error:        "connection timeout",
			TraceData:    nil,
			SSLInfo:      nil,
		},
	}
	
	return &MonitoringSession{
		StartTime: startTime,
		EndTime:   endTime,
		Config:    config,
		Data:      data,
	}
}

func TestPrometheusExporter_Export(t *testing.T) {
	// Create test monitoring session
	session := createTestMonitoringSession()
	
	// Test Prometheus export
	exporter := NewPrometheusExporter()
	var buf bytes.Buffer
	
	err := exporter.Export(session, &buf)
	if err != nil {
		t.Fatalf("Prometheus export failed: %v", err)
	}
	
	output := buf.String()
	
	// Verify Prometheus format structure
	if !strings.Contains(output, "# Netirk Network Monitoring Metrics") {
		t.Error("Prometheus output missing header comment")
	}
	
	// Check for essential metrics
	expectedMetrics := []string{
		"netirk_response_time_seconds",
		"netirk_success_rate",
		"netirk_checks_total",
		"netirk_up",
		"netirk_ssl_cert_expiry_days",
		"netirk_ssl_cert_valid",
		"netirk_dns_duration_seconds",
		"netirk_connect_duration_seconds",
		"netirk_tls_duration_seconds",
		"netirk_first_byte_duration_seconds",
	}
	
	for _, metric := range expectedMetrics {
		if !strings.Contains(output, metric) {
			t.Errorf("Prometheus output missing metric: %s", metric)
		}
	}
	
	// Check for HELP and TYPE comments
	if !strings.Contains(output, "# HELP netirk_response_time_seconds") {
		t.Error("Prometheus output missing response time HELP comment")
	}
	
	if !strings.Contains(output, "# TYPE netirk_response_time_seconds gauge") {
		t.Error("Prometheus output missing response time TYPE comment")
	}
	
	// Check for target labels
	if !strings.Contains(output, "target=\"https://example.com\"") {
		t.Error("Prometheus output missing target label")
	}
	
	// Check for metric values
	if !strings.Contains(output, "netirk_up{target=\"https://example.com\"} 1") {
		t.Error("Prometheus output missing up metric for successful target")
	}
	
	if !strings.Contains(output, "netirk_up{target=\"https://test.com\"} 0") {
		t.Error("Prometheus output missing up metric for failed target")
	}
	
	// Check for SSL metrics
	if !strings.Contains(output, "netirk_ssl_cert_expiry_days{target=\"https://example.com\",issuer=\"Let_s Encrypt\"} 30") {
		t.Error("Prometheus output missing SSL expiry metric")
	}
	
	// Check for success rate
	if !strings.Contains(output, "netirk_success_rate{target=\"https://example.com\"} 100.00") {
		t.Error("Prometheus output missing success rate for successful target")
	}
	
	if !strings.Contains(output, "netirk_success_rate{target=\"https://test.com\"} 0.00") {
		t.Error("Prometheus output missing success rate for failed target")
	}
}

func TestPrometheusExporter_ExportEmptySession(t *testing.T) {
	// Create empty monitoring session
	session := &MonitoringSession{
		StartTime: time.Now().Add(-1 * time.Hour),
		EndTime:   time.Now(),
		Config: MonitoringConfig{
			Targets:  []string{},
			Interval: 30 * time.Second,
		},
		Data: []MonitoringData{},
	}
	
	// Test Prometheus export with empty data
	exporter := NewPrometheusExporter()
	var buf bytes.Buffer
	
	err := exporter.Export(session, &buf)
	if err != nil {
		t.Fatalf("Prometheus export failed for empty session: %v", err)
	}
	
	output := buf.String()
	
	// Should still contain header
	if !strings.Contains(output, "# Netirk Network Monitoring Metrics") {
		t.Error("Prometheus output missing header comment")
	}
	
	// Should contain metric definitions but no data points
	if !strings.Contains(output, "# HELP netirk_response_time_seconds") {
		t.Error("Prometheus output missing metric definitions")
	}
	
	// Should not contain any actual metric values
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "netirk_") && !strings.HasPrefix(line, "# ") {
			t.Errorf("Unexpected metric line in empty session: %s", line)
		}
	}
}

func TestPrometheusExporter_GetContentType(t *testing.T) {
	exporter := NewPrometheusExporter()
	contentType := exporter.GetContentType()
	
	if contentType != "text/plain; version=0.0.4" {
		t.Errorf("Expected content type 'text/plain; version=0.0.4', got '%s'", contentType)
	}
}

func TestPrometheusExporter_GetFileExtension(t *testing.T) {
	exporter := NewPrometheusExporter()
	extension := exporter.GetFileExtension()
	
	if extension != ".prom" {
		t.Errorf("Expected file extension '.prom', got '%s'", extension)
	}
}

func TestPrometheusExporter_SanitizeLabel(t *testing.T) {
	exporter := NewPrometheusExporter()
	
	testCases := []struct {
		input    string
		expected string
	}{
		{"https://example.com", "https://example.com"},
		{"test\"label", "test_label"},
		{"test\\label", "test_label"},
		{"test\nlabel", "test_label"},
		{"test\rlabel", "test_label"},
		{"Let's Encrypt", "Let_s Encrypt"},
	}
	
	for _, tc := range testCases {
		result := exporter.sanitizeLabel(tc.input)
		if result != tc.expected {
			t.Errorf("sanitizeLabel(%q) = %q, expected %q", tc.input, result, tc.expected)
		}
	}
}

func TestPrometheusExporter_CalculateTargetMetrics(t *testing.T) {
	session := createTestMonitoringSession()
	exporter := NewPrometheusExporter()
	
	targetMetrics := exporter.calculateTargetMetrics(session)
	
	// Check that we have metrics for both targets
	if len(targetMetrics) != 2 {
		t.Errorf("Expected metrics for 2 targets, got %d", len(targetMetrics))
	}
	
	// Check metrics for successful target
	exampleMetrics, ok := targetMetrics["https://example.com"]
	if !ok {
		t.Fatal("Missing metrics for https://example.com")
	}
	
	if exampleMetrics.TotalChecks != 1 {
		t.Errorf("Expected 1 total check for example.com, got %d", exampleMetrics.TotalChecks)
	}
	
	if exampleMetrics.SuccessfulChecks != 1 {
		t.Errorf("Expected 1 successful check for example.com, got %d", exampleMetrics.SuccessfulChecks)
	}
	
	if exampleMetrics.FailedChecks != 0 {
		t.Errorf("Expected 0 failed checks for example.com, got %d", exampleMetrics.FailedChecks)
	}
	
	if exampleMetrics.LastStatus != "success" {
		t.Errorf("Expected last status 'success' for example.com, got '%s'", exampleMetrics.LastStatus)
	}
	
	if exampleMetrics.AvgResponseTime != 250.0 {
		t.Errorf("Expected avg response time 250ms for example.com, got %.2f", exampleMetrics.AvgResponseTime)
	}
	
	// Check metrics for failed target
	testMetrics, ok := targetMetrics["https://test.com"]
	if !ok {
		t.Fatal("Missing metrics for https://test.com")
	}
	
	if testMetrics.TotalChecks != 1 {
		t.Errorf("Expected 1 total check for test.com, got %d", testMetrics.TotalChecks)
	}
	
	if testMetrics.SuccessfulChecks != 0 {
		t.Errorf("Expected 0 successful checks for test.com, got %d", testMetrics.SuccessfulChecks)
	}
	
	if testMetrics.FailedChecks != 1 {
		t.Errorf("Expected 1 failed check for test.com, got %d", testMetrics.FailedChecks)
	}
	
	if testMetrics.LastStatus != "failure" {
		t.Errorf("Expected last status 'failure' for test.com, got '%s'", testMetrics.LastStatus)
	}
}

func TestGetExporter_Prometheus(t *testing.T) {
	// Test prometheus format
	prometheusExporter, err := GetExporter("prometheus")
	if err != nil {
		t.Errorf("Failed to get Prometheus exporter: %v", err)
	}
	if prometheusExporter == nil {
		t.Error("Prometheus exporter is nil")
	}
	
	// Test case insensitive
	prometheusExporter2, err := GetExporter("PROMETHEUS")
	if err != nil {
		t.Errorf("Failed to get Prometheus exporter with uppercase: %v", err)
	}
	if prometheusExporter2 == nil {
		t.Error("Prometheus exporter with uppercase is nil")
	}
}