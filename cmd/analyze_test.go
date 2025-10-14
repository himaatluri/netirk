package cmd

import (
	"math"
	"testing"
	"time"

	"github.com/himasagaratluri/netirk/cmd/helpers"
)

func TestAnalyzeTarget(t *testing.T) {
	// Create test data
	testData := []helpers.MonitoringData{
		{
			Timestamp:    time.Now().Add(-3 * time.Minute),
			Target:       "https://example.com",
			Status:       "success",
			ResponseTime: 200 * time.Millisecond,
			StatusCode:   200,
		},
		{
			Timestamp:    time.Now().Add(-2 * time.Minute),
			Target:       "https://example.com",
			Status:       "success",
			ResponseTime: 300 * time.Millisecond,
			StatusCode:   200,
		},
		{
			Timestamp:    time.Now().Add(-1 * time.Minute),
			Target:       "https://example.com",
			Status:       "failure",
			ResponseTime: 0,
			Error:        "connection timeout",
		},
	}

	result := analyzeTarget("https://example.com", testData, false, false)

	// Verify basic statistics
	if result.Target != "https://example.com" {
		t.Errorf("Expected target 'https://example.com', got '%s'", result.Target)
	}

	if result.TotalChecks != 3 {
		t.Errorf("Expected 3 total checks, got %d", result.TotalChecks)
	}

	if result.FailureCount != 1 {
		t.Errorf("Expected 1 failure, got %d", result.FailureCount)
	}

	expectedSuccessRate := 66.67 // 2 out of 3 successful
	if result.SuccessRate < expectedSuccessRate-0.1 || result.SuccessRate > expectedSuccessRate+0.1 {
		t.Errorf("Expected success rate around %.2f%%, got %.2f%%", expectedSuccessRate, result.SuccessRate)
	}

	// Verify response time calculations (only for successful requests)
	expectedAvg := 250 * time.Millisecond // (200 + 300) / 2
	if result.AvgResponseTime != expectedAvg {
		t.Errorf("Expected average response time %v, got %v", expectedAvg, result.AvgResponseTime)
	}

	if result.MinResponseTime != 200*time.Millisecond {
		t.Errorf("Expected min response time 200ms, got %v", result.MinResponseTime)
	}

	if result.MaxResponseTime != 300*time.Millisecond {
		t.Errorf("Expected max response time 300ms, got %v", result.MaxResponseTime)
	}
}

func TestCalculatePercentile(t *testing.T) {
	// Test with sorted response times
	times := []time.Duration{
		100 * time.Millisecond,
		200 * time.Millisecond,
		300 * time.Millisecond,
		400 * time.Millisecond,
		500 * time.Millisecond,
	}

	// Test 95th percentile (should be close to 480ms)
	p95 := calculatePercentile(times, 95)
	expected95 := 480 * time.Millisecond
	if p95 != expected95 {
		t.Errorf("Expected 95th percentile %v, got %v", expected95, p95)
	}

	// Test 50th percentile (median, should be 300ms)
	p50 := calculatePercentile(times, 50)
	expected50 := 300 * time.Millisecond
	if p50 != expected50 {
		t.Errorf("Expected 50th percentile %v, got %v", expected50, p50)
	}

	// Test edge case with single value
	singleTime := []time.Duration{250 * time.Millisecond}
	p95Single := calculatePercentile(singleTime, 95)
	if p95Single != 250*time.Millisecond {
		t.Errorf("Expected single value percentile 250ms, got %v", p95Single)
	}

	// Test edge case with empty slice
	emptyTimes := []time.Duration{}
	p95Empty := calculatePercentile(emptyTimes, 95)
	if p95Empty != 0 {
		t.Errorf("Expected empty slice percentile 0, got %v", p95Empty)
	}
}

func TestAnalyzeSession(t *testing.T) {
	// Create test session
	session := &helpers.MonitoringSession{
		StartTime: time.Now().Add(-5 * time.Minute),
		EndTime:   time.Now(),
		Config: helpers.MonitoringConfig{
			Targets:  []string{"https://example.com", "https://google.com"},
			Interval: 30 * time.Second,
		},
		Data: []helpers.MonitoringData{
			{
				Timestamp:    time.Now().Add(-4 * time.Minute),
				Target:       "https://example.com",
				Status:       "success",
				ResponseTime: 200 * time.Millisecond,
				StatusCode:   200,
			},
			{
				Timestamp:    time.Now().Add(-3 * time.Minute),
				Target:       "https://google.com",
				Status:       "success",
				ResponseTime: 150 * time.Millisecond,
				StatusCode:   200,
			},
			{
				Timestamp:    time.Now().Add(-2 * time.Minute),
				Target:       "https://example.com",
				Status:       "failure",
				ResponseTime: 0,
				Error:        "timeout",
			},
		},
	}

	analysis := analyzeSession(session, "", false, false)

	// Verify session-level statistics
	if analysis.TotalTargets != 2 {
		t.Errorf("Expected 2 targets, got %d", analysis.TotalTargets)
	}

	if analysis.TotalChecks != 3 {
		t.Errorf("Expected 3 total checks, got %d", analysis.TotalChecks)
	}

	expectedOverallSuccess := 66.67 // 2 out of 3 successful
	if analysis.OverallSuccess < expectedOverallSuccess-0.1 || analysis.OverallSuccess > expectedOverallSuccess+0.1 {
		t.Errorf("Expected overall success rate around %.2f%%, got %.2f%%", expectedOverallSuccess, analysis.OverallSuccess)
	}

	// Verify that results are sorted by target name
	if len(analysis.Results) != 2 {
		t.Errorf("Expected 2 target results, got %d", len(analysis.Results))
	}

	// Results should be sorted alphabetically
	if analysis.Results[0].Target != "https://example.com" {
		t.Errorf("Expected first result to be 'https://example.com', got '%s'", analysis.Results[0].Target)
	}

	if analysis.Results[1].Target != "https://google.com" {
		t.Errorf("Expected second result to be 'https://google.com', got '%s'", analysis.Results[1].Target)
	}
}

func TestAnalyzeSessionWithTargetFilter(t *testing.T) {
	// Create test session with multiple targets
	session := &helpers.MonitoringSession{
		StartTime: time.Now().Add(-5 * time.Minute),
		EndTime:   time.Now(),
		Data: []helpers.MonitoringData{
			{
				Timestamp:    time.Now().Add(-4 * time.Minute),
				Target:       "https://example.com",
				Status:       "success",
				ResponseTime: 200 * time.Millisecond,
			},
			{
				Timestamp:    time.Now().Add(-3 * time.Minute),
				Target:       "https://google.com",
				Status:       "success",
				ResponseTime: 150 * time.Millisecond,
			},
		},
	}

	// Analyze with target filter
	analysis := analyzeSession(session, "https://example.com", false, false)

	// Should only include the filtered target
	if analysis.TotalTargets != 1 {
		t.Errorf("Expected 1 target with filter, got %d", analysis.TotalTargets)
	}

	if analysis.TotalChecks != 1 {
		t.Errorf("Expected 1 check with filter, got %d", analysis.TotalChecks)
	}

	if len(analysis.Results) != 1 {
		t.Errorf("Expected 1 result with filter, got %d", len(analysis.Results))
	}

	if analysis.Results[0].Target != "https://example.com" {
		t.Errorf("Expected filtered result to be 'https://example.com', got '%s'", analysis.Results[0].Target)
	}
}

func TestCalculateTrends(t *testing.T) {
	// Create test data with a clear upward trend in response times
	testData := []helpers.MonitoringData{}
	baseTime := time.Now().Add(-1 * time.Hour)
	
	for i := 0; i < 20; i++ {
		// Response time increases over time (degrading trend)
		responseTime := time.Duration(100+i*10) * time.Millisecond
		testData = append(testData, helpers.MonitoringData{
			Timestamp:    baseTime.Add(time.Duration(i*3) * time.Minute),
			Target:       "https://example.com",
			Status:       "success",
			ResponseTime: responseTime,
			StatusCode:   200,
		})
	}
	
	trends := calculateTrends(testData)
	
	// Should detect response time trend
	if len(trends) == 0 {
		t.Error("Expected to detect trends, but none found")
		return
	}
	
	var responseTrend *TrendData
	for _, trend := range trends {
		if trend.Metric == "response_time" {
			responseTrend = &trend
			break
		}
	}
	
	if responseTrend == nil {
		t.Error("Expected to find response time trend")
		return
	}
	
	if responseTrend.Direction != "degrading" {
		t.Errorf("Expected degrading trend, got %s", responseTrend.Direction)
	}
	
	if responseTrend.Slope <= 0 {
		t.Errorf("Expected positive slope for degrading trend, got %f", responseTrend.Slope)
	}
}

func TestDetectAnomalies(t *testing.T) {
	// Create test data with normal response times and one anomaly
	testData := []helpers.MonitoringData{}
	baseTime := time.Now().Add(-30 * time.Minute)
	
	// Add normal data points (around 200ms)
	for i := 0; i < 15; i++ {
		responseTime := time.Duration(190+i*2) * time.Millisecond
		testData = append(testData, helpers.MonitoringData{
			Timestamp:    baseTime.Add(time.Duration(i*2) * time.Minute),
			Target:       "https://example.com",
			Status:       "success",
			ResponseTime: responseTime,
			StatusCode:   200,
		})
	}
	
	// Add an anomalous data point (very high response time)
	testData = append(testData, helpers.MonitoringData{
		Timestamp:    baseTime.Add(30 * time.Minute),
		Target:       "https://example.com",
		Status:       "success",
		ResponseTime: 2000 * time.Millisecond, // Much higher than normal
		StatusCode:   200,
	})
	
	anomalies := detectAnomalies(testData)
	
	if len(anomalies) == 0 {
		t.Error("Expected to detect anomalies, but none found")
		return
	}
	
	// Should detect the high response time anomaly
	var responseAnomaly *AnomalyData
	for _, anomaly := range anomalies {
		if anomaly.Metric == "response_time" {
			responseAnomaly = &anomaly
			break
		}
	}
	
	if responseAnomaly == nil {
		t.Error("Expected to find response time anomaly")
		return
	}
	
	if responseAnomaly.Value != 2000 {
		t.Errorf("Expected anomaly value 2000, got %f", responseAnomaly.Value)
	}
	
	if responseAnomaly.Deviation < 2.0 {
		t.Errorf("Expected deviation > 2.0, got %f", responseAnomaly.Deviation)
	}
}

func TestComparePeriods(t *testing.T) {
	// Create test data with different performance in two periods
	testData := []helpers.MonitoringData{}
	baseTime := time.Now().Add(-2 * time.Hour)
	
	// First period: good performance (100ms average)
	for i := 0; i < 10; i++ {
		testData = append(testData, helpers.MonitoringData{
			Timestamp:    baseTime.Add(time.Duration(i*5) * time.Minute),
			Target:       "https://example.com",
			Status:       "success",
			ResponseTime: time.Duration(95+i) * time.Millisecond,
			StatusCode:   200,
		})
	}
	
	// Second period: worse performance (200ms average)
	for i := 0; i < 10; i++ {
		testData = append(testData, helpers.MonitoringData{
			Timestamp:    baseTime.Add(time.Duration(60+i*5) * time.Minute),
			Target:       "https://example.com",
			Status:       "success",
			ResponseTime: time.Duration(195+i) * time.Millisecond,
			StatusCode:   200,
		})
	}
	
	comparisons := comparePeriodsData(testData)
	
	if len(comparisons) == 0 {
		t.Error("Expected period comparison, but none found")
		return
	}
	
	comparison := comparisons[0]
	
	// Should detect significant performance degradation
	if comparison.ResponseChange <= 0 {
		t.Errorf("Expected positive response change (degradation), got %f", comparison.ResponseChange)
	}
	
	if comparison.ResponseChange < 50 { // Should be around 100% increase
		t.Errorf("Expected significant response change, got %f%%", comparison.ResponseChange)
	}
	
	if comparison.Significance == "negligible" {
		t.Errorf("Expected significant change, got %s", comparison.Significance)
	}
}

func TestLinearRegression(t *testing.T) {
	// Test with perfect linear relationship: y = 2x + 1
	x := []float64{1, 2, 3, 4, 5}
	y := []float64{3, 5, 7, 9, 11}
	
	slope, rSquared := linearRegression(x, y)
	
	// Should detect slope of 2
	if math.Abs(slope-2.0) > 0.001 {
		t.Errorf("Expected slope 2.0, got %f", slope)
	}
	
	// Should have perfect correlation (R² = 1)
	if math.Abs(rSquared-1.0) > 0.001 {
		t.Errorf("Expected R² = 1.0, got %f", rSquared)
	}
}

func TestCalculateMeanAndStdDev(t *testing.T) {
	values := []float64{1, 2, 3, 4, 5}
	
	mean := calculateMean(values)
	expectedMean := 3.0
	if math.Abs(mean-expectedMean) > 0.001 {
		t.Errorf("Expected mean %f, got %f", expectedMean, mean)
	}
	
	stdDev := calculateStdDev(values, mean)
	expectedStdDev := math.Sqrt(2.5) // sqrt((1+1+0+1+1)/4)
	if math.Abs(stdDev-expectedStdDev) > 0.001 {
		t.Errorf("Expected std dev %f, got %f", expectedStdDev, stdDev)
	}
}

func TestAnalyzeTargetWithTrends(t *testing.T) {
	// Create test data with enough points for trend analysis and period comparison
	testData := []helpers.MonitoringData{}
	baseTime := time.Now().Add(-60 * time.Minute)
	
	// Need at least 20 data points for period comparison
	for i := 0; i < 25; i++ {
		testData = append(testData, helpers.MonitoringData{
			Timestamp:    baseTime.Add(time.Duration(i*2) * time.Minute),
			Target:       "https://example.com",
			Status:       "success",
			ResponseTime: time.Duration(100+i*3) * time.Millisecond,
			StatusCode:   200,
		})
	}
	
	result := analyzeTarget("https://example.com", testData, true, true)
	
	// Should include trends and comparisons
	if len(result.Trends) == 0 {
		t.Error("Expected trends to be calculated when showTrends=true")
	}
	
	if len(result.Comparisons) == 0 {
		t.Error("Expected comparisons to be calculated when comparePeriods=true")
	}
}