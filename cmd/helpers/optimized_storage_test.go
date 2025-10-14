package helpers

import (
	"log"
	"os"
	"testing"
	"time"
)

func TestNewOptimizedStorage(t *testing.T) {
	config := MonitoringConfig{
		Targets:  []string{"https://example.com"},
		Interval: 30 * time.Second,
		Duration: 1 * time.Hour,
	}
	
	perfManager := NewPerformanceManager(DefaultPerformanceConfig(), nil)
	defer perfManager.Cleanup()
	
	logger := log.New(os.Stdout, "TEST: ", log.LstdFlags)
	
	storage := NewOptimizedStorage(config, perfManager, logger)
	if storage == nil {
		t.Fatal("NewOptimizedStorage returned nil")
	}
	
	// Test that it implements DataStorage interface
	var _ DataStorage = storage
	
	// Test initial state
	session := storage.GetSession()
	if session == nil {
		t.Fatal("GetSession returned nil")
	}
	
	if len(session.Config.Targets) != 1 {
		t.Errorf("Expected 1 target, got %d", len(session.Config.Targets))
	}
	
	if session.Config.Interval != 30*time.Second {
		t.Errorf("Expected interval 30s, got %v", session.Config.Interval)
	}
	
	if len(session.Data) != 0 {
		t.Errorf("Expected empty data initially, got %d items", len(session.Data))
	}
}

func TestNewOptimizedStorage_NilParameters(t *testing.T) {
	config := MonitoringConfig{
		Targets:  []string{"https://example.com"},
		Interval: 30 * time.Second,
	}
	
	// Test with nil performance manager
	storage := NewOptimizedStorage(config, nil, nil)
	if storage == nil {
		t.Fatal("NewOptimizedStorage should handle nil parameters gracefully")
	}
	
	// Should still function as basic storage
	session := storage.GetSession()
	if session == nil {
		t.Fatal("GetSession returned nil with nil parameters")
	}
}

func TestOptimizedStorage_Store(t *testing.T) {
	config := MonitoringConfig{
		Targets:  []string{"https://example.com"},
		Interval: 30 * time.Second,
	}
	
	perfManager := NewPerformanceManager(DefaultPerformanceConfig(), nil)
	defer perfManager.Cleanup()
	
	logger := log.New(os.Stdout, "TEST: ", log.LstdFlags)
	storage := NewOptimizedStorage(config, perfManager, logger)
	
	// Test storing data
	data := &MonitoringData{
		Timestamp:    time.Now(),
		Target:       "https://example.com",
		Status:       "success",
		ResponseTime: 200 * time.Millisecond,
		StatusCode:   200,
	}
	
	err := storage.Store(data)
	if err != nil {
		t.Fatalf("Store failed: %v", err)
	}
	
	// Verify data was stored
	session := storage.GetSession()
	if len(session.Data) != 1 {
		t.Errorf("Expected 1 data point, got %d", len(session.Data))
	}
	
	if session.Data[0].Target != data.Target {
		t.Errorf("Expected target %s, got %s", data.Target, session.Data[0].Target)
	}
}

func TestOptimizedStorage_Store_NilData(t *testing.T) {
	config := MonitoringConfig{
		Targets:  []string{"https://example.com"},
		Interval: 30 * time.Second,
	}
	
	storage := NewOptimizedStorage(config, nil, nil)
	
	err := storage.Store(nil)
	if err == nil {
		t.Error("Expected error when storing nil data")
	}
}

func TestOptimizedStorage_GetLatestData(t *testing.T) {
	config := MonitoringConfig{
		Targets:  []string{"https://example.com", "https://test.com"},
		Interval: 30 * time.Second,
	}
	
	storage := NewOptimizedStorage(config, nil, nil)
	
	// Store multiple data points for different targets
	data1 := &MonitoringData{
		Timestamp: time.Now().Add(-2 * time.Minute),
		Target:    "https://example.com",
		Status:    "success",
	}
	
	data2 := &MonitoringData{
		Timestamp: time.Now().Add(-1 * time.Minute),
		Target:    "https://test.com",
		Status:    "failed",
	}
	
	data3 := &MonitoringData{
		Timestamp: time.Now(),
		Target:    "https://example.com",
		Status:    "success",
	}
	
	storage.Store(data1)
	storage.Store(data2)
	storage.Store(data3)
	
	// Get latest data for example.com (should be data3)
	latest := storage.GetLatestData("https://example.com")
	if latest == nil {
		t.Fatal("GetLatestData returned nil")
	}
	
	if latest.Timestamp != data3.Timestamp {
		t.Error("Expected latest data to be the most recent")
	}
	
	// Get latest data for test.com (should be data2)
	latest = storage.GetLatestData("https://test.com")
	if latest == nil {
		t.Fatal("GetLatestData returned nil for test.com")
	}
	
	if latest.Status != "failed" {
		t.Errorf("Expected status 'failed', got %s", latest.Status)
	}
	
	// Get latest data for non-existent target
	latest = storage.GetLatestData("https://nonexistent.com")
	if latest != nil {
		t.Error("Expected nil for non-existent target")
	}
}

func TestOptimizedStorage_GetBufferSize(t *testing.T) {
	config := MonitoringConfig{
		Targets:  []string{"https://example.com"},
		Interval: 30 * time.Second,
	}
	
	storage := NewOptimizedStorage(config, nil, nil)
	
	// Initial buffer size should be 0
	if size := storage.GetBufferSize(); size != 0 {
		t.Errorf("Expected initial buffer size 0, got %d", size)
	}
	
	// Add data
	data := &MonitoringData{
		Timestamp: time.Now(),
		Target:    "https://example.com",
		Status:    "success",
	}
	
	storage.Store(data)
	
	if size := storage.GetBufferSize(); size != 1 {
		t.Errorf("Expected buffer size 1 after storing data, got %d", size)
	}
}

func TestOptimizedStorage_GetStorageStats(t *testing.T) {
	config := MonitoringConfig{
		Targets:  []string{"https://example.com"},
		Interval: 30 * time.Second,
	}
	
	storage := NewOptimizedStorage(config, nil, nil)
	
	// Get initial stats
	stats := storage.GetStorageStats()
	if stats["buffer_size"].(int) != 0 {
		t.Errorf("Expected initial buffer size 0, got %d", stats["buffer_size"])
	}
	
	// Add data
	data := &MonitoringData{
		Timestamp: time.Now(),
		Target:    "https://example.com",
		Status:    "success",
	}
	
	storage.Store(data)
	
	stats = storage.GetStorageStats()
	if stats["buffer_size"].(int) != 1 {
		t.Errorf("Expected buffer size 1 after storing data, got %d", stats["buffer_size"])
	}
}

func TestOptimizedStorage_ClearData(t *testing.T) {
	config := MonitoringConfig{
		Targets:  []string{"https://example.com"},
		Interval: 30 * time.Second,
	}
	
	storage := NewOptimizedStorage(config, nil, nil)
	
	// Add data
	data := &MonitoringData{
		Timestamp: time.Now(),
		Target:    "https://example.com",
		Status:    "success",
	}
	
	storage.Store(data)
	
	// Verify data exists
	if storage.GetBufferSize() != 1 {
		t.Error("Expected data to be stored")
	}
	
	// Clear data
	storage.ClearData()
	
	// Verify data is cleared
	if storage.GetBufferSize() != 0 {
		t.Error("Expected data to be cleared")
	}
	
	session := storage.GetSession()
	if !session.EndTime.IsZero() {
		t.Error("Expected EndTime to be reset after clear")
	}
}

func TestOptimizedStorage_SetEndTime(t *testing.T) {
	config := MonitoringConfig{
		Targets:  []string{"https://example.com"},
		Interval: 30 * time.Second,
	}
	
	storage := NewOptimizedStorage(config, nil, nil)
	
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

func TestOptimizedStorage_SaveToFile(t *testing.T) {
	config := MonitoringConfig{
		Targets:  []string{"https://example.com"},
		Interval: 30 * time.Second,
	}
	
	storage := NewOptimizedStorage(config, nil, nil)
	
	// Add test data
	data := &MonitoringData{
		Timestamp:    time.Now(),
		Target:       "https://example.com",
		Status:       "success",
		ResponseTime: 150 * time.Millisecond,
		StatusCode:   200,
	}
	
	storage.Store(data)
	
	// Save to temporary file
	tmpFile := "test_optimized_storage.json"
	defer os.Remove(tmpFile)
	
	err := storage.SaveToFile(tmpFile)
	if err != nil {
		t.Fatalf("SaveToFile failed: %v", err)
	}
	
	// Verify file exists
	if _, err := os.Stat(tmpFile); os.IsNotExist(err) {
		t.Fatal("File was not created")
	}
}

func TestOptimizedStorage_LoadFromFile(t *testing.T) {
	config := MonitoringConfig{
		Targets:  []string{"https://example.com"},
		Interval: 30 * time.Second,
	}
	
	// Create and save data first
	storage1 := NewOptimizedStorage(config, nil, nil)
	data := &MonitoringData{
		Timestamp:    time.Now().Truncate(time.Second),
		Target:       "https://example.com",
		Status:       "success",
		ResponseTime: 150 * time.Millisecond,
		StatusCode:   200,
	}
	
	storage1.Store(data)
	
	tmpFile := "test_optimized_load.json"
	defer os.Remove(tmpFile)
	
	err := storage1.SaveToFile(tmpFile)
	if err != nil {
		t.Fatalf("SaveToFile failed: %v", err)
	}
	
	// Create new storage and load from file
	storage2 := NewOptimizedStorage(MonitoringConfig{}, nil, nil)
	err = storage2.LoadFromFile(tmpFile)
	if err != nil {
		t.Fatalf("LoadFromFile failed: %v", err)
	}
	
	// Verify loaded data
	session := storage2.GetSession()
	if len(session.Data) != 1 {
		t.Errorf("Expected 1 data point after loading, got %d", len(session.Data))
	}
	
	if session.Data[0].Target != data.Target {
		t.Errorf("Expected target %s, got %s", data.Target, session.Data[0].Target)
	}
}

func TestOptimizedStorage_FlushToFile(t *testing.T) {
	config := MonitoringConfig{
		Targets:  []string{"https://example.com"},
		Interval: 30 * time.Second,
	}
	
	storage := NewOptimizedStorage(config, nil, nil)
	
	// Add test data
	data := &MonitoringData{
		Timestamp: time.Now(),
		Target:    "https://example.com",
		Status:    "success",
	}
	
	storage.Store(data)
	
	// Test flush without clearing buffer
	filename1 := "test_flush_no_clear.json"
	defer os.Remove(filename1)
	
	err := storage.FlushToFile(filename1, false)
	if err != nil {
		t.Fatalf("FlushToFile failed: %v", err)
	}
	
	// Buffer should still contain data
	if storage.GetBufferSize() != 1 {
		t.Error("Expected buffer to retain data after flush without clear")
	}
	
	// Test flush with clearing buffer
	filename2 := "test_flush_clear.json"
	defer os.Remove(filename2)
	
	err = storage.FlushToFile(filename2, true)
	if err != nil {
		t.Fatalf("FlushToFile with clear failed: %v", err)
	}
	
	// Buffer should be cleared
	if storage.GetBufferSize() != 0 {
		t.Error("Expected buffer to be cleared after flush with clear")
	}
}

func TestOptimizedStorage_GetDataForTarget(t *testing.T) {
	config := MonitoringConfig{
		Targets:  []string{"https://example.com", "https://test.com"},
		Interval: 30 * time.Second,
	}
	
	storage := NewOptimizedStorage(config, nil, nil)
	
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

func TestOptimizedStorage_GetDataSince(t *testing.T) {
	config := MonitoringConfig{
		Targets:  []string{"https://example.com"},
		Interval: 30 * time.Second,
	}
	
	storage := NewOptimizedStorage(config, nil, nil)
	
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

func TestOptimizedStorage_PerformanceIntegration(t *testing.T) {
	config := MonitoringConfig{
		Targets:  []string{"https://example.com"},
		Interval: 30 * time.Second,
	}
	
	perfManager := NewPerformanceManager(DefaultPerformanceConfig(), nil)
	defer perfManager.Cleanup()
	
	logger := log.New(os.Stdout, "TEST: ", log.LstdFlags)
	storage := NewOptimizedStorage(config, perfManager, logger)
	
	// Add multiple data points to test performance tracking
	for i := 0; i < 10; i++ {
		data := &MonitoringData{
			Timestamp:    time.Now().Add(time.Duration(i) * time.Second),
			Target:       "https://example.com",
			Status:       "success",
			ResponseTime: time.Duration(100+i*10) * time.Millisecond,
			StatusCode:   200,
		}
		
		err := storage.Store(data)
		if err != nil {
			t.Fatalf("Store failed on iteration %d: %v", i, err)
		}
	}
	
	// Check that all data was stored
	session := storage.GetSession()
	if len(session.Data) != 10 {
		t.Errorf("Expected 10 data points, got %d", len(session.Data))
	}
	
	// Check performance metrics if available
	if perfManager != nil {
		utilization := perfManager.GetResourceUtilization()
		if utilization["memory_usage_mb"].(float64) <= 0 {
			t.Error("Expected positive memory usage")
		}
	}
}

func TestOptimizedStorage_ConcurrentAccess(t *testing.T) {
	config := MonitoringConfig{
		Targets:  []string{"https://example.com"},
		Interval: 30 * time.Second,
	}
	
	storage := NewOptimizedStorage(config, nil, nil)
	
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
	session := storage.GetSession()
	if len(session.Data) != 10 {
		t.Errorf("Expected 10 data points after concurrent writes, got %d", len(session.Data))
	}
}

func TestOptimizedStorage_MemoryOptimization(t *testing.T) {
	config := MonitoringConfig{
		Targets:  []string{"https://example.com"},
		Interval: 30 * time.Second,
	}
	
	// Create performance manager with low memory limit
	perfConfig := PerformanceConfig{
		MaxConcurrentTargets: 10,
		ConnectionPoolSize:   10,
		MemoryLimitMB:       1, // Very low limit
		GCInterval:          1 * time.Minute,
		MetricsInterval:     30 * time.Second,
	}
	perfManager := NewPerformanceManager(perfConfig, nil)
	defer perfManager.Cleanup()
	
	logger := log.New(os.Stdout, "TEST: ", log.LstdFlags)
	storage := NewOptimizedStorage(config, perfManager, logger)
	
	// Add data that might trigger memory optimization
	for i := 0; i < 100; i++ {
		data := &MonitoringData{
			Timestamp:    time.Now().Add(time.Duration(i) * time.Second),
			Target:       "https://example.com",
			Status:       "success",
			ResponseTime: time.Duration(100+i) * time.Millisecond,
			StatusCode:   200,
		}
		
		err := storage.Store(data)
		if err != nil {
			t.Fatalf("Store failed on iteration %d: %v", i, err)
		}
	}
	
	// Check that storage still functions despite memory constraints
	session := storage.GetSession()
	if len(session.Data) != 100 {
		t.Errorf("Expected 100 data points, got %d", len(session.Data))
	}
	
	// Check memory usage
	withinLimit, memoryUsage := perfManager.CheckMemoryUsage()
	t.Logf("Memory usage: %.2f MB, within limit: %v", memoryUsage, withinLimit)
	
	// Storage should still function even if memory limit is exceeded
	if memoryUsage <= 0 {
		t.Error("Expected positive memory usage")
	}
}