package helpers

import (
	"encoding/json"
	"os"
	"testing"
	"time"
)

func TestMonitoringData_JSONSerialization(t *testing.T) {
	// Test data serialization and deserialization
	originalData := MonitoringData{
		Timestamp:    time.Now().Truncate(time.Second), // Truncate for comparison
		Target:       "https://example.com",
		Status:       "success",
		ResponseTime: 250 * time.Millisecond,
		StatusCode:   200,
		SSLInfo: &SSLData{
			ExpiryDate:   time.Now().AddDate(0, 6, 0).Truncate(time.Second),
			DaysToExpiry: 180,
			Issuer:       "Let's Encrypt",
			IsValid:      true,
		},
		TraceData: &NetworkTrace{
			DNSTime:       10 * time.Millisecond,
			ConnectTime:   50 * time.Millisecond,
			TLSTime:       100 * time.Millisecond,
			FirstByteTime: 250 * time.Millisecond,
		},
	}

	// Serialize to JSON
	jsonData, err := json.Marshal(originalData)
	if err != nil {
		t.Fatalf("Failed to marshal MonitoringData: %v", err)
	}

	// Deserialize from JSON
	var deserializedData MonitoringData
	err = json.Unmarshal(jsonData, &deserializedData)
	if err != nil {
		t.Fatalf("Failed to unmarshal MonitoringData: %v", err)
	}

	// Compare key fields
	if deserializedData.Target != originalData.Target {
		t.Errorf("Target mismatch: got %s, want %s", deserializedData.Target, originalData.Target)
	}
	if deserializedData.Status != originalData.Status {
		t.Errorf("Status mismatch: got %s, want %s", deserializedData.Status, originalData.Status)
	}
	if deserializedData.StatusCode != originalData.StatusCode {
		t.Errorf("StatusCode mismatch: got %d, want %d", deserializedData.StatusCode, originalData.StatusCode)
	}
}

func TestNewInMemoryStorage(t *testing.T) {
	config := MonitoringConfig{
		Targets:  []string{"https://example.com", "https://google.com"},
		Interval: 30 * time.Second,
		Duration: 1 * time.Hour,
	}

	storage := NewInMemoryStorage(config)

	if storage == nil {
		t.Fatal("NewInMemoryStorage returned nil")
	}

	session := storage.GetSession()
	if session == nil {
		t.Fatal("GetSession returned nil")
	}

	if len(session.Config.Targets) != 2 {
		t.Errorf("Expected 2 targets, got %d", len(session.Config.Targets))
	}

	if session.Config.Interval != 30*time.Second {
		t.Errorf("Expected interval 30s, got %v", session.Config.Interval)
	}

	if len(session.Data) != 0 {
		t.Errorf("Expected empty data slice, got %d items", len(session.Data))
	}
}

func TestInMemoryStorage_Store(t *testing.T) {
	config := MonitoringConfig{
		Targets:  []string{"https://example.com"},
		Interval: 30 * time.Second,
	}

	storage := NewInMemoryStorage(config)

	// Test storing valid data
	data := &MonitoringData{
		Timestamp:    time.Now(),
		Target:       "https://example.com",
		Status:       "success",
		ResponseTime: 200 * time.Millisecond,
		StatusCode:   200,
	}

	err := storage.Store(data)
	if err != nil {
		t.Fatalf("Failed to store data: %v", err)
	}

	// Verify data was stored
	session := storage.GetSession()
	if len(session.Data) != 1 {
		t.Errorf("Expected 1 data point, got %d", len(session.Data))
	}

	if session.Data[0].Target != data.Target {
		t.Errorf("Target mismatch: got %s, want %s", session.Data[0].Target, data.Target)
	}

	// Test storing nil data
	err = storage.Store(nil)
	if err == nil {
		t.Error("Expected error when storing nil data, got nil")
	}
}

func TestInMemoryStorage_GetLatestData(t *testing.T) {
	config := MonitoringConfig{
		Targets:  []string{"https://example.com", "https://google.com"},
		Interval: 30 * time.Second,
	}

	storage := NewInMemoryStorage(config)

	// Store multiple data points for different targets
	data1 := &MonitoringData{
		Timestamp: time.Now().Add(-2 * time.Minute),
		Target:    "https://example.com",
		Status:    "success",
	}

	data2 := &MonitoringData{
		Timestamp: time.Now().Add(-1 * time.Minute),
		Target:    "https://google.com",
		Status:    "success",
	}

	data3 := &MonitoringData{
		Timestamp: time.Now(),
		Target:    "https://example.com",
		Status:    "failed",
	}

	storage.Store(data1)
	storage.Store(data2)
	storage.Store(data3)

	// Get latest data for example.com (should be data3)
	latest := storage.GetLatestData("https://example.com")
	if latest == nil {
		t.Fatal("GetLatestData returned nil")
	}

	if latest.Status != "failed" {
		t.Errorf("Expected latest status 'failed', got '%s'", latest.Status)
	}

	// Get latest data for google.com (should be data2)
	latest = storage.GetLatestData("https://google.com")
	if latest == nil {
		t.Fatal("GetLatestData returned nil for google.com")
	}

	if latest.Status != "success" {
		t.Errorf("Expected latest status 'success', got '%s'", latest.Status)
	}

	// Get latest data for non-existent target
	latest = storage.GetLatestData("https://nonexistent.com")
	if latest != nil {
		t.Error("Expected nil for non-existent target, got data")
	}
}

func TestInMemoryStorage_SaveAndLoadFile(t *testing.T) {
	config := MonitoringConfig{
		Targets:  []string{"https://example.com"},
		Interval: 30 * time.Second,
		Duration: 1 * time.Hour,
	}

	storage := NewInMemoryStorage(config)

	// Add some test data
	data := &MonitoringData{
		Timestamp:    time.Now().Truncate(time.Second),
		Target:       "https://example.com",
		Status:       "success",
		ResponseTime: 150 * time.Millisecond,
		StatusCode:   200,
	}

	storage.Store(data)

	// Save to temporary file
	tempFile := "test_monitoring_data.json"
	defer os.Remove(tempFile) // Clean up

	err := storage.SaveToFile(tempFile)
	if err != nil {
		t.Fatalf("Failed to save to file: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(tempFile); os.IsNotExist(err) {
		t.Fatal("File was not created")
	}

	// Create new storage and load from file
	newStorage := NewInMemoryStorage(MonitoringConfig{})
	err = newStorage.LoadFromFile(tempFile)
	if err != nil {
		t.Fatalf("Failed to load from file: %v", err)
	}

	// Verify loaded data
	loadedSession := newStorage.GetSession()
	if len(loadedSession.Data) != 1 {
		t.Errorf("Expected 1 data point after loading, got %d", len(loadedSession.Data))
	}

	if loadedSession.Data[0].Target != data.Target {
		t.Errorf("Target mismatch after loading: got %s, want %s",
			loadedSession.Data[0].Target, data.Target)
	}

	if loadedSession.Config.Interval != config.Interval {
		t.Errorf("Interval mismatch after loading: got %v, want %v",
			loadedSession.Config.Interval, config.Interval)
	}
}

func TestInMemoryStorage_ClearData(t *testing.T) {
	config := MonitoringConfig{
		Targets:  []string{"https://example.com"},
		Interval: 30 * time.Second,
	}

	storage := NewInMemoryStorage(config)

	// Add some test data
	data := &MonitoringData{
		Timestamp: time.Now(),
		Target:    "https://example.com",
		Status:    "success",
	}

	storage.Store(data)

	// Verify data exists
	if storage.GetDataCount() != 1 {
		t.Errorf("Expected 1 data point, got %d", storage.GetDataCount())
	}

	// Clear data
	storage.ClearData()

	// Verify data is cleared
	if storage.GetDataCount() != 0 {
		t.Errorf("Expected 0 data points after clear, got %d", storage.GetDataCount())
	}

	session := storage.GetSession()
	if !session.EndTime.IsZero() {
		t.Error("Expected EndTime to be reset after clear")
	}
}

func TestInMemoryStorage_ConcurrentAccess(t *testing.T) {
	config := MonitoringConfig{
		Targets:  []string{"https://example.com"},
		Interval: 30 * time.Second,
	}

	storage := NewInMemoryStorage(config)

	// Test concurrent writes
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func(id int) {
			data := &MonitoringData{
				Timestamp: time.Now(),
				Target:    "https://example.com",
				Status:    "success",
			}
			storage.Store(data)
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify all data was stored
	if storage.GetDataCount() != 10 {
		t.Errorf("Expected 10 data points after concurrent writes, got %d", storage.GetDataCount())
	}
}

func TestSSLData_Validation(t *testing.T) {
	sslData := SSLData{
		ExpiryDate:   time.Now().AddDate(0, 0, 30), // 30 days from now
		DaysToExpiry: 30,
		Issuer:       "Let's Encrypt",
		IsValid:      true,
	}

	// Test JSON serialization
	jsonData, err := json.Marshal(sslData)
	if err != nil {
		t.Fatalf("Failed to marshal SSLData: %v", err)
	}

	var deserializedSSL SSLData
	err = json.Unmarshal(jsonData, &deserializedSSL)
	if err != nil {
		t.Fatalf("Failed to unmarshal SSLData: %v", err)
	}

	if deserializedSSL.Issuer != sslData.Issuer {
		t.Errorf("Issuer mismatch: got %s, want %s", deserializedSSL.Issuer, sslData.Issuer)
	}

	if deserializedSSL.IsValid != sslData.IsValid {
		t.Errorf("IsValid mismatch: got %t, want %t", deserializedSSL.IsValid, sslData.IsValid)
	}
}

func TestNetworkTrace_Validation(t *testing.T) {
	trace := NetworkTrace{
		DNSTime:       10 * time.Millisecond,
		ConnectTime:   50 * time.Millisecond,
		TLSTime:       100 * time.Millisecond,
		FirstByteTime: 200 * time.Millisecond,
	}

	// Test JSON serialization
	jsonData, err := json.Marshal(trace)
	if err != nil {
		t.Fatalf("Failed to marshal NetworkTrace: %v", err)
	}

	var deserializedTrace NetworkTrace
	err = json.Unmarshal(jsonData, &deserializedTrace)
	if err != nil {
		t.Fatalf("Failed to unmarshal NetworkTrace: %v", err)
	}

	if deserializedTrace.DNSTime != trace.DNSTime {
		t.Errorf("DNSTime mismatch: got %v, want %v", deserializedTrace.DNSTime, trace.DNSTime)
	}

	if deserializedTrace.FirstByteTime != trace.FirstByteTime {
		t.Errorf("FirstByteTime mismatch: got %v, want %v", deserializedTrace.FirstByteTime, trace.FirstByteTime)
	}
}

func TestInMemoryStorage_BufferManagement(t *testing.T) {
	config := MonitoringConfig{
		Targets:  []string{"https://example.com"},
		Interval: 30 * time.Second,
	}
	storage := NewInMemoryStorage(config)

	// Test initial buffer size
	if storage.GetBufferSize() != 0 {
		t.Errorf("Expected initial buffer size to be 0, got %d", storage.GetBufferSize())
	}

	// Add data and check buffer size
	data := &MonitoringData{
		Timestamp:    time.Now(),
		Target:       "https://example.com",
		Status:       "success",
		ResponseTime: 100 * time.Millisecond,
	}

	storage.Store(data)
	if storage.GetBufferSize() != 1 {
		t.Errorf("Expected buffer size to be 1, got %d", storage.GetBufferSize())
	}

	// Add more data
	storage.Store(data)
	if storage.GetBufferSize() != 2 {
		t.Errorf("Expected buffer size to be 2, got %d", storage.GetBufferSize())
	}
}

func TestInMemoryStorage_FlushToFile(t *testing.T) {
	config := MonitoringConfig{
		Targets:  []string{"https://example.com"},
		Interval: 30 * time.Second,
	}
	storage := NewInMemoryStorage(config)

	// Add test data
	data := &MonitoringData{
		Timestamp:    time.Now(),
		Target:       "https://example.com",
		Status:       "success",
		ResponseTime: 100 * time.Millisecond,
	}
	storage.Store(data)

	// Test flush to file without clearing buffer
	filename := "test_flush_no_clear.json"
	defer os.Remove(filename)

	err := storage.FlushToFile(filename, false)
	if err != nil {
		t.Fatalf("Failed to flush to file: %v", err)
	}

	// Buffer should still contain data
	if storage.GetBufferSize() != 1 {
		t.Errorf("Expected buffer size to be 1 after flush without clear, got %d", storage.GetBufferSize())
	}

	// Test flush to file with clearing buffer
	filename2 := "test_flush_clear.json"
	defer os.Remove(filename2)

	err = storage.FlushToFile(filename2, true)
	if err != nil {
		t.Fatalf("Failed to flush to file with clear: %v", err)
	}

	// Buffer should be cleared
	if storage.GetBufferSize() != 0 {
		t.Errorf("Expected buffer size to be 0 after flush with clear, got %d", storage.GetBufferSize())
	}

	// Verify both files were created
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		t.Error("Expected first file to be created after flush")
	}
	if _, err := os.Stat(filename2); os.IsNotExist(err) {
		t.Error("Expected second file to be created after flush")
	}
}

func TestInMemoryStorage_GetDataForTarget(t *testing.T) {
	config := MonitoringConfig{
		Targets:  []string{"https://example.com", "https://test.com"},
		Interval: 30 * time.Second,
	}
	storage := NewInMemoryStorage(config)

	// Add data for multiple targets
	testData := []*MonitoringData{
		{
			Timestamp: time.Now(),
			Target:    "https://example.com",
			Status:    "success",
		},
		{
			Timestamp: time.Now().Add(30 * time.Second),
			Target:    "https://test.com",
			Status:    "failed",
		},
		{
			Timestamp: time.Now().Add(60 * time.Second),
			Target:    "https://example.com",
			Status:    "success",
		},
	}

	for _, data := range testData {
		storage.Store(data)
	}

	// Get data for specific target
	exampleData := storage.GetDataForTarget("https://example.com")
	if len(exampleData) != 2 {
		t.Errorf("Expected 2 data points for example.com, got %d", len(exampleData))
	}

	testData2 := storage.GetDataForTarget("https://test.com")
	if len(testData2) != 1 {
		t.Errorf("Expected 1 data point for test.com, got %d", len(testData2))
	}

	// Test non-existent target
	nonExistent := storage.GetDataForTarget("https://nonexistent.com")
	if len(nonExistent) != 0 {
		t.Errorf("Expected 0 data points for non-existent target, got %d", len(nonExistent))
	}
}

func TestInMemoryStorage_GetDataSince(t *testing.T) {
	config := MonitoringConfig{
		Targets:  []string{"https://example.com"},
		Interval: 30 * time.Second,
	}
	storage := NewInMemoryStorage(config)

	baseTime := time.Now()
	
	// Add data with different timestamps
	testData := []*MonitoringData{
		{
			Timestamp: baseTime.Add(-2 * time.Hour),
			Target:    "https://example.com",
			Status:    "success",
		},
		{
			Timestamp: baseTime.Add(-1 * time.Hour),
			Target:    "https://example.com",
			Status:    "failed",
		},
		{
			Timestamp: baseTime.Add(-30 * time.Minute),
			Target:    "https://example.com",
			Status:    "success",
		},
	}

	for _, data := range testData {
		storage.Store(data)
	}

	// Get data since 1.5 hours ago
	since := baseTime.Add(-90 * time.Minute)
	recentData := storage.GetDataSince(since)
	
	// Should get the last 2 data points
	if len(recentData) != 2 {
		t.Errorf("Expected 2 data points since %v, got %d", since, len(recentData))
	}

	// Get data since 3 hours ago (should get all data)
	since = baseTime.Add(-3 * time.Hour)
	allData := storage.GetDataSince(since)
	if len(allData) != 3 {
		t.Errorf("Expected 3 data points since %v, got %d", since, len(allData))
	}

	// Get data since now (should get no data)
	since = baseTime
	noData := storage.GetDataSince(since)
	if len(noData) != 0 {
		t.Errorf("Expected 0 data points since %v, got %d", since, len(noData))
	}
}

func TestInMemoryStorage_SetEndTime(t *testing.T) {
	config := MonitoringConfig{
		Targets:  []string{"https://example.com"},
		Interval: 30 * time.Second,
	}
	storage := NewInMemoryStorage(config)

	// Initially end time should be zero
	session := storage.GetSession()
	if !session.EndTime.IsZero() {
		t.Error("Expected initial end time to be zero")
	}

	// Set end time
	endTime := time.Now()
	storage.SetEndTime(endTime)

	// Verify end time was set
	session = storage.GetSession()
	if session.EndTime.IsZero() {
		t.Error("Expected end time to be set")
	}

	// Allow for small time differences due to processing
	timeDiff := session.EndTime.Sub(endTime)
	if timeDiff > time.Second || timeDiff < -time.Second {
		t.Errorf("End time differs too much from expected: %v", timeDiff)
	}
}
