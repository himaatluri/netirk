package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"sort"
	"time"

	"github.com/himasagaratluri/netirk/cmd/helpers"
	"github.com/spf13/cobra"
)

// TrendData represents trend analysis for a specific metric over time
type TrendData struct {
	Metric    string    `json:"metric"`
	Direction string    `json:"direction"` // "improving", "degrading", "stable"
	Slope     float64   `json:"slope"`     // Rate of change per hour
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	Confidence float64  `json:"confidence"` // 0-1, how confident we are in the trend
}

// AnomalyData represents detected anomalies in the monitoring data
type AnomalyData struct {
	Timestamp   time.Time     `json:"timestamp"`
	Metric      string        `json:"metric"`
	Value       float64       `json:"value"`
	Expected    float64       `json:"expected"`
	Deviation   float64       `json:"deviation"`   // How many standard deviations from normal
	Severity    string        `json:"severity"`    // "low", "medium", "high"
	Description string        `json:"description"`
}

// PeriodComparison represents comparison between two time periods
type PeriodComparison struct {
	Period1Start    time.Time     `json:"period1_start"`
	Period1End      time.Time     `json:"period1_end"`
	Period2Start    time.Time     `json:"period2_start"`
	Period2End      time.Time     `json:"period2_end"`
	AvgResponseP1   time.Duration `json:"avg_response_p1"`
	AvgResponseP2   time.Duration `json:"avg_response_p2"`
	SuccessRateP1   float64       `json:"success_rate_p1"`
	SuccessRateP2   float64       `json:"success_rate_p2"`
	ResponseChange  float64       `json:"response_change_percent"`
	SuccessChange   float64       `json:"success_change_percent"`
	Significance    string        `json:"significance"` // "significant", "minor", "negligible"
}

// AnalysisResult represents the statistical analysis results for a target
type AnalysisResult struct {
	Target          string             `json:"target"`
	TotalChecks     int                `json:"total_checks"`
	SuccessRate     float64            `json:"success_rate"`
	AvgResponseTime time.Duration      `json:"avg_response_time"`
	MinResponseTime time.Duration      `json:"min_response_time"`
	MaxResponseTime time.Duration      `json:"max_response_time"`
	P95ResponseTime time.Duration      `json:"p95_response_time"`
	P99ResponseTime time.Duration      `json:"p99_response_time"`
	FailureCount    int                `json:"failure_count"`
	FirstCheck      time.Time          `json:"first_check"`
	LastCheck       time.Time          `json:"last_check"`
	Trends          []TrendData        `json:"trends,omitempty"`
	Anomalies       []AnomalyData      `json:"anomalies,omitempty"`
	Comparisons     []PeriodComparison `json:"comparisons,omitempty"`
}

// SessionAnalysis represents the complete analysis of a monitoring session
type SessionAnalysis struct {
	SessionStart    time.Time        `json:"session_start"`
	SessionEnd      time.Time        `json:"session_end"`
	SessionDuration time.Duration    `json:"session_duration"`
	TotalTargets    int              `json:"total_targets"`
	TotalChecks     int              `json:"total_checks"`
	OverallSuccess  float64          `json:"overall_success_rate"`
	Results         []AnalysisResult `json:"results"`
}

var analyzeCmd = &cobra.Command{
	Use:   "analyze",
	Short: "Analyze historical monitoring data",
	Long: `Analyze historical monitoring data from JSON files to generate comprehensive
performance statistics, success rates, response time metrics, trends, and anomalies.

The analyze command provides detailed insights into your monitoring data including:
• Statistical analysis (average, min, max, percentiles)
• Success rate calculations and failure analysis
• Performance trend detection over time
• Anomaly detection for unusual patterns
• Period-to-period performance comparisons
• Multiple output formats for integration with other tools

Analysis Features:
• Response Time Analysis: Calculate average, minimum, maximum, and percentile response times
• Success Rate Tracking: Monitor uptime and failure patterns across targets
• Trend Analysis: Identify improving or degrading performance over time
• Anomaly Detection: Highlight unusual response times or failure clusters
• Period Comparison: Compare performance between different time periods
• Target Filtering: Focus analysis on specific targets

Examples:
  # Basic analysis of monitoring data
  netirk analyze --data-file monitoring-session.json

  # Analyze specific target only
  netirk analyze --data-file monitoring.json --target "https://api.example.com"

  # Include trend analysis and anomaly detection
  netirk analyze --data-file monitoring.json --show-trends

  # Compare performance between time periods
  netirk analyze --data-file monitoring.json --compare-periods

  # Export analysis results as JSON
  netirk analyze --data-file monitoring.json --output json

  # Comprehensive analysis with all features
  netirk analyze --data-file monitoring.json --show-trends --compare-periods --output json

Output Formats:
  • Human-readable (default): Formatted tables and summaries
  • JSON: Machine-readable format for integration with other tools

The JSON output includes detailed statistics, trend data, anomalies, and comparisons
that can be used for automated reporting or integration with monitoring dashboards.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Check global flags
		verbose, _ := rootCmd.PersistentFlags().GetBool("verbose")
		quiet, _ := rootCmd.PersistentFlags().GetBool("quiet")
		
		// Set log level based on verbose/quiet flags
		if verbose {
			log.SetFlags(log.LstdFlags | log.Lshortfile)
		} else if quiet {
			log.SetOutput(io.Discard)
		}
		
		dataFile, _ := cmd.Flags().GetString("data-file")
		outputFormat, _ := cmd.Flags().GetString("output")
		target, _ := cmd.Flags().GetString("target")
		showTrends, _ := cmd.Flags().GetBool("show-trends")
		comparePeriods, _ := cmd.Flags().GetBool("compare-periods")
		
		// Check if global output flag is set and no local output is specified
		if outputFormat == "" {
			if globalOutput, _ := rootCmd.PersistentFlags().GetString("output"); globalOutput != "" {
				outputFormat = globalOutput
			}
		}
		
		if dataFile == "" {
			fmt.Println("Error: --data-file is required")
			os.Exit(1)
		}

		// Load monitoring data from file
		session, err := loadMonitoringData(dataFile)
		if err != nil {
			fmt.Printf("Error loading data file: %v\n", err)
			os.Exit(1)
		}

		// Perform analysis
		analysis := analyzeSession(session, target, showTrends, comparePeriods)

		// Output results
		switch outputFormat {
		case "json":
			outputJSON(analysis)
		default:
			outputHumanReadable(analysis)
		}
	},
}

// loadMonitoringData loads monitoring session data from a JSON file
func loadMonitoringData(filename string) (*helpers.MonitoringSession, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read data file: %w", err)
	}

	var session helpers.MonitoringSession
	err = json.Unmarshal(data, &session)
	if err != nil {
		return nil, fmt.Errorf("failed to parse JSON data: %w", err)
	}

	return &session, nil
}

// analyzeSession performs statistical analysis on the monitoring session
func analyzeSession(session *helpers.MonitoringSession, targetFilter string, showTrends bool, comparePeriods bool) *SessionAnalysis {
	analysis := &SessionAnalysis{
		SessionStart: session.StartTime,
		SessionEnd:   session.EndTime,
		Results:      make([]AnalysisResult, 0),
	}

	if !session.EndTime.IsZero() {
		analysis.SessionDuration = session.EndTime.Sub(session.StartTime)
	}

	// Group data by target
	targetData := make(map[string][]helpers.MonitoringData)
	for _, data := range session.Data {
		// Apply target filter if specified
		if targetFilter != "" && data.Target != targetFilter {
			continue
		}
		
		if _, exists := targetData[data.Target]; !exists {
			targetData[data.Target] = make([]helpers.MonitoringData, 0)
		}
		targetData[data.Target] = append(targetData[data.Target], data)
	}

	analysis.TotalTargets = len(targetData)
	totalChecks := 0
	totalSuccesses := 0

	// Analyze each target
	for target, data := range targetData {
		result := analyzeTarget(target, data, showTrends, comparePeriods)
		analysis.Results = append(analysis.Results, result)
		
		totalChecks += result.TotalChecks
		totalSuccesses += result.TotalChecks - result.FailureCount
	}

	analysis.TotalChecks = totalChecks
	if totalChecks > 0 {
		analysis.OverallSuccess = float64(totalSuccesses) / float64(totalChecks) * 100
	}

	// Sort results by target name for consistent output
	sort.Slice(analysis.Results, func(i, j int) bool {
		return analysis.Results[i].Target < analysis.Results[j].Target
	})

	return analysis
}

// analyzeTarget performs statistical analysis for a single target
func analyzeTarget(target string, data []helpers.MonitoringData, showTrends bool, comparePeriods bool) AnalysisResult {
	result := AnalysisResult{
		Target:      target,
		TotalChecks: len(data),
	}

	if len(data) == 0 {
		return result
	}

	// Sort data by timestamp for chronological analysis
	sort.Slice(data, func(i, j int) bool {
		return data[i].Timestamp.Before(data[j].Timestamp)
	})

	result.FirstCheck = data[0].Timestamp
	result.LastCheck = data[len(data)-1].Timestamp

	// Collect response times and count successes/failures
	var responseTimes []time.Duration
	successCount := 0

	for _, point := range data {
		if point.Status == "success" {
			successCount++
			responseTimes = append(responseTimes, point.ResponseTime)
		} else {
			result.FailureCount++
		}
	}

	// Calculate success rate
	result.SuccessRate = float64(successCount) / float64(result.TotalChecks) * 100

	// Calculate response time statistics (only for successful requests)
	if len(responseTimes) > 0 {
		// Sort response times for percentile calculations
		sort.Slice(responseTimes, func(i, j int) bool {
			return responseTimes[i] < responseTimes[j]
		})

		result.MinResponseTime = responseTimes[0]
		result.MaxResponseTime = responseTimes[len(responseTimes)-1]

		// Calculate average
		var total time.Duration
		for _, rt := range responseTimes {
			total += rt
		}
		result.AvgResponseTime = total / time.Duration(len(responseTimes))

		// Calculate percentiles
		result.P95ResponseTime = calculatePercentile(responseTimes, 95)
		result.P99ResponseTime = calculatePercentile(responseTimes, 99)
	}

	// Perform trend analysis if requested
	if showTrends && len(data) > 5 { // Need minimum data points for meaningful trends
		result.Trends = calculateTrends(data)
		result.Anomalies = detectAnomalies(data)
	}

	// Perform period comparison if requested
	if comparePeriods && len(data) > 10 { // Need sufficient data for comparison
		result.Comparisons = comparePeriodsData(data)
	}

	return result
}

// calculatePercentile calculates the specified percentile from sorted response times
func calculatePercentile(sortedTimes []time.Duration, percentile float64) time.Duration {
	if len(sortedTimes) == 0 {
		return 0
	}

	if len(sortedTimes) == 1 {
		return sortedTimes[0]
	}

	// Calculate index for percentile
	index := (percentile / 100.0) * float64(len(sortedTimes)-1)
	
	// If index is exact, return that element
	if index == math.Floor(index) {
		return sortedTimes[int(index)]
	}

	// Interpolate between two values
	lower := int(math.Floor(index))
	upper := int(math.Ceil(index))
	
	if upper >= len(sortedTimes) {
		return sortedTimes[len(sortedTimes)-1]
	}

	// Linear interpolation
	weight := index - math.Floor(index)
	lowerTime := float64(sortedTimes[lower])
	upperTime := float64(sortedTimes[upper])
	
	interpolated := lowerTime + weight*(upperTime-lowerTime)
	return time.Duration(interpolated)
}

// outputJSON outputs the analysis results in JSON format
func outputJSON(analysis *SessionAnalysis) {
	data, err := json.MarshalIndent(analysis, "", "  ")
	if err != nil {
		fmt.Printf("Error formatting JSON output: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(string(data))
}

// outputHumanReadable outputs the analysis results in human-readable format
func outputHumanReadable(analysis *SessionAnalysis) {
	fmt.Println("=== Monitoring Session Analysis ===")
	fmt.Printf("Session Duration: %v\n", analysis.SessionDuration.Round(time.Second))
	fmt.Printf("Total Targets: %d\n", analysis.TotalTargets)
	fmt.Printf("Total Checks: %d\n", analysis.TotalChecks)
	fmt.Printf("Overall Success Rate: %.2f%%\n\n", analysis.OverallSuccess)

	for _, result := range analysis.Results {
		fmt.Printf("Target: %s\n", result.Target)
		fmt.Printf("  Total Checks: %d\n", result.TotalChecks)
		fmt.Printf("  Success Rate: %.2f%% (%d failures)\n", result.SuccessRate, result.FailureCount)
		
		if result.AvgResponseTime > 0 {
			fmt.Printf("  Response Times:\n")
			fmt.Printf("    Average: %v\n", result.AvgResponseTime.Round(time.Millisecond))
			fmt.Printf("    Min: %v\n", result.MinResponseTime.Round(time.Millisecond))
			fmt.Printf("    Max: %v\n", result.MaxResponseTime.Round(time.Millisecond))
			fmt.Printf("    95th percentile: %v\n", result.P95ResponseTime.Round(time.Millisecond))
			fmt.Printf("    99th percentile: %v\n", result.P99ResponseTime.Round(time.Millisecond))
		}
		
		fmt.Printf("  Time Range: %v to %v\n", 
			result.FirstCheck.Format("2006-01-02 15:04:05"),
			result.LastCheck.Format("2006-01-02 15:04:05"))

		// Display trends if available
		if len(result.Trends) > 0 {
			fmt.Printf("  Trends:\n")
			for _, trend := range result.Trends {
				fmt.Printf("    %s: %s", trend.Metric, trend.Direction)
				if trend.Direction != "stable" {
					if trend.Metric == "response_time" {
						fmt.Printf(" (%.1f ms/hour)", trend.Slope)
					} else {
						fmt.Printf(" (%.2f%%/hour)", trend.Slope)
					}
				}
				fmt.Printf(" [confidence: %.2f]\n", trend.Confidence)
			}
		}

		// Display anomalies if available
		if len(result.Anomalies) > 0 {
			fmt.Printf("  Anomalies Detected:\n")
			for _, anomaly := range result.Anomalies {
				fmt.Printf("    %s [%s]: %s\n", 
					anomaly.Timestamp.Format("15:04:05"), 
					anomaly.Severity, 
					anomaly.Description)
			}
		}

		// Display period comparisons if available
		if len(result.Comparisons) > 0 {
			fmt.Printf("  Period Comparison:\n")
			for _, comp := range result.Comparisons {
				fmt.Printf("    Period 1: %v avg response, %.1f%% success\n", 
					comp.AvgResponseP1.Round(time.Millisecond), comp.SuccessRateP1)
				fmt.Printf("    Period 2: %v avg response, %.1f%% success\n", 
					comp.AvgResponseP2.Round(time.Millisecond), comp.SuccessRateP2)
				fmt.Printf("    Changes: %.1f%% response time, %.1f%% success rate [%s]\n", 
					comp.ResponseChange, comp.SuccessChange, comp.Significance)
			}
		}
		
		fmt.Println()
	}
}

// calculateTrends analyzes performance trends over time
func calculateTrends(data []helpers.MonitoringData) []TrendData {
	var trends []TrendData
	
	if len(data) < 5 {
		return trends
	}

	// Calculate response time trend
	responseTrend := calculateResponseTimeTrend(data)
	if responseTrend != nil {
		trends = append(trends, *responseTrend)
	}

	// Calculate success rate trend
	successTrend := calculateSuccessRateTrend(data)
	if successTrend != nil {
		trends = append(trends, *successTrend)
	}

	return trends
}

// calculateResponseTimeTrend calculates trend for response times using linear regression
func calculateResponseTimeTrend(data []helpers.MonitoringData) *TrendData {
	var timePoints []float64
	var responseValues []float64
	
	startTime := data[0].Timestamp
	
	for _, point := range data {
		if point.Status == "success" && point.ResponseTime > 0 {
			// Convert time to hours since start for easier slope interpretation
			hours := point.Timestamp.Sub(startTime).Hours()
			timePoints = append(timePoints, hours)
			responseValues = append(responseValues, float64(point.ResponseTime.Milliseconds()))
		}
	}
	
	if len(timePoints) < 3 {
		return nil
	}
	
	slope, confidence := linearRegression(timePoints, responseValues)
	
	direction := "stable"
	if math.Abs(slope) > 1.0 { // More than 1ms change per hour
		if slope > 0 {
			direction = "degrading"
		} else {
			direction = "improving"
		}
	}
	
	return &TrendData{
		Metric:     "response_time",
		Direction:  direction,
		Slope:      slope,
		StartTime:  data[0].Timestamp,
		EndTime:    data[len(data)-1].Timestamp,
		Confidence: confidence,
	}
}

// calculateSuccessRateTrend calculates trend for success rates over time windows
func calculateSuccessRateTrend(data []helpers.MonitoringData) *TrendData {
	if len(data) < 10 {
		return nil
	}
	
	// Calculate success rate in time windows
	windowSize := len(data) / 5 // 5 windows
	if windowSize < 2 {
		windowSize = 2
	}
	
	var timePoints []float64
	var successRates []float64
	startTime := data[0].Timestamp
	
	for i := 0; i < len(data)-windowSize; i += windowSize {
		end := i + windowSize
		if end > len(data) {
			end = len(data)
		}
		
		window := data[i:end]
		successCount := 0
		for _, point := range window {
			if point.Status == "success" {
				successCount++
			}
		}
		
		successRate := float64(successCount) / float64(len(window)) * 100
		hours := window[len(window)/2].Timestamp.Sub(startTime).Hours()
		
		timePoints = append(timePoints, hours)
		successRates = append(successRates, successRate)
	}
	
	if len(timePoints) < 3 {
		return nil
	}
	
	slope, confidence := linearRegression(timePoints, successRates)
	
	direction := "stable"
	if math.Abs(slope) > 0.5 { // More than 0.5% change per hour
		if slope > 0 {
			direction = "improving"
		} else {
			direction = "degrading"
		}
	}
	
	return &TrendData{
		Metric:     "success_rate",
		Direction:  direction,
		Slope:      slope,
		StartTime:  data[0].Timestamp,
		EndTime:    data[len(data)-1].Timestamp,
		Confidence: confidence,
	}
}

// linearRegression performs simple linear regression and returns slope and R-squared
func linearRegression(x, y []float64) (slope float64, rSquared float64) {
	if len(x) != len(y) || len(x) < 2 {
		return 0, 0
	}
	
	n := float64(len(x))
	
	// Calculate means
	var sumX, sumY float64
	for i := 0; i < len(x); i++ {
		sumX += x[i]
		sumY += y[i]
	}
	meanX := sumX / n
	meanY := sumY / n
	
	// Calculate slope and correlation
	var numerator, denomX, denomY float64
	for i := 0; i < len(x); i++ {
		dx := x[i] - meanX
		dy := y[i] - meanY
		numerator += dx * dy
		denomX += dx * dx
		denomY += dy * dy
	}
	
	if denomX == 0 {
		return 0, 0
	}
	
	slope = numerator / denomX
	
	// Calculate R-squared
	if denomY == 0 {
		rSquared = 0
	} else {
		correlation := numerator / math.Sqrt(denomX*denomY)
		rSquared = correlation * correlation
	}
	
	return slope, rSquared
}

// detectAnomalies identifies unusual patterns in monitoring data
func detectAnomalies(data []helpers.MonitoringData) []AnomalyData {
	var anomalies []AnomalyData
	
	if len(data) < 10 {
		return anomalies
	}
	
	// Detect response time anomalies
	responseAnomalies := detectResponseTimeAnomalies(data)
	anomalies = append(anomalies, responseAnomalies...)
	
	// Detect failure clusters
	failureAnomalies := detectFailureClusters(data)
	anomalies = append(anomalies, failureAnomalies...)
	
	return anomalies
}

// detectResponseTimeAnomalies finds response times that are unusually high
func detectResponseTimeAnomalies(data []helpers.MonitoringData) []AnomalyData {
	var anomalies []AnomalyData
	var responseTimes []float64
	
	// Collect successful response times
	for _, point := range data {
		if point.Status == "success" && point.ResponseTime > 0 {
			responseTimes = append(responseTimes, float64(point.ResponseTime.Milliseconds()))
		}
	}
	
	if len(responseTimes) < 5 {
		return anomalies
	}
	
	// Calculate mean and standard deviation
	mean := calculateMean(responseTimes)
	stdDev := calculateStdDev(responseTimes, mean)
	
	if stdDev == 0 {
		return anomalies
	}
	
	// Find anomalies (values more than 2 standard deviations from mean)
	for _, point := range data {
		if point.Status == "success" && point.ResponseTime > 0 {
			value := float64(point.ResponseTime.Milliseconds())
			deviation := math.Abs(value-mean) / stdDev
			
			if deviation > 2.0 {
				severity := "medium"
				if deviation > 3.0 {
					severity = "high"
				}
				
				anomalies = append(anomalies, AnomalyData{
					Timestamp:   point.Timestamp,
					Metric:      "response_time",
					Value:       value,
					Expected:    mean,
					Deviation:   deviation,
					Severity:    severity,
					Description: fmt.Sprintf("Response time %.0fms is %.1f standard deviations above normal", value, deviation),
				})
			}
		}
	}
	
	return anomalies
}

// detectFailureClusters identifies periods with unusually high failure rates
func detectFailureClusters(data []helpers.MonitoringData) []AnomalyData {
	var anomalies []AnomalyData
	
	if len(data) < 10 {
		return anomalies
	}
	
	// Use sliding window to detect failure clusters
	windowSize := 5
	if len(data) < 20 {
		windowSize = 3
	}
	
	for i := 0; i <= len(data)-windowSize; i++ {
		window := data[i : i+windowSize]
		failureCount := 0
		
		for _, point := range window {
			if point.Status != "success" {
				failureCount++
			}
		}
		
		failureRate := float64(failureCount) / float64(windowSize)
		
		// If more than 60% failures in window, it's an anomaly
		if failureRate > 0.6 {
			severity := "medium"
			if failureRate > 0.8 {
				severity = "high"
			}
			
			anomalies = append(anomalies, AnomalyData{
				Timestamp:   window[windowSize/2].Timestamp,
				Metric:      "failure_rate",
				Value:       failureRate * 100,
				Expected:    20.0, // Assume normal failure rate is around 20%
				Deviation:   (failureRate - 0.2) / 0.2,
				Severity:    severity,
				Description: fmt.Sprintf("Failure cluster detected: %.0f%% failures in %d consecutive checks", failureRate*100, windowSize),
			})
		}
	}
	
	return anomalies
}

// comparePeriodsData splits data into periods and compares performance
func comparePeriodsData(data []helpers.MonitoringData) []PeriodComparison {
	var comparisons []PeriodComparison
	
	if len(data) < 20 {
		return comparisons
	}
	
	// Split data into two equal periods
	midpoint := len(data) / 2
	period1 := data[:midpoint]
	period2 := data[midpoint:]
	
	// Calculate metrics for each period
	p1Stats := calculatePeriodStats(period1)
	p2Stats := calculatePeriodStats(period2)
	
	// Calculate percentage changes
	responseChange := 0.0
	if p1Stats.avgResponse > 0 {
		responseChange = ((p2Stats.avgResponse - p1Stats.avgResponse) / p1Stats.avgResponse) * 100
	}
	
	successChange := p2Stats.successRate - p1Stats.successRate
	
	// Determine significance
	significance := "negligible"
	if math.Abs(responseChange) > 10 || math.Abs(successChange) > 5 {
		significance = "minor"
	}
	if math.Abs(responseChange) > 25 || math.Abs(successChange) > 15 {
		significance = "significant"
	}
	
	comparison := PeriodComparison{
		Period1Start:   period1[0].Timestamp,
		Period1End:     period1[len(period1)-1].Timestamp,
		Period2Start:   period2[0].Timestamp,
		Period2End:     period2[len(period2)-1].Timestamp,
		AvgResponseP1:  time.Duration(p1Stats.avgResponse * float64(time.Millisecond)),
		AvgResponseP2:  time.Duration(p2Stats.avgResponse * float64(time.Millisecond)),
		SuccessRateP1:  p1Stats.successRate,
		SuccessRateP2:  p2Stats.successRate,
		ResponseChange: responseChange,
		SuccessChange:  successChange,
		Significance:   significance,
	}
	
	comparisons = append(comparisons, comparison)
	return comparisons
}

// periodStats holds basic statistics for a time period
type periodStats struct {
	avgResponse float64
	successRate float64
}

// calculatePeriodStats calculates basic statistics for a period of data
func calculatePeriodStats(data []helpers.MonitoringData) periodStats {
	if len(data) == 0 {
		return periodStats{}
	}
	
	var totalResponse float64
	successCount := 0
	responseCount := 0
	
	for _, point := range data {
		if point.Status == "success" {
			successCount++
			if point.ResponseTime > 0 {
				totalResponse += float64(point.ResponseTime.Milliseconds())
				responseCount++
			}
		}
	}
	
	avgResponse := 0.0
	if responseCount > 0 {
		avgResponse = totalResponse / float64(responseCount)
	}
	
	successRate := float64(successCount) / float64(len(data)) * 100
	
	return periodStats{
		avgResponse: avgResponse,
		successRate: successRate,
	}
}

// calculateMean calculates the arithmetic mean of a slice of float64 values
func calculateMean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	
	var sum float64
	for _, v := range values {
		sum += v
	}
	
	return sum / float64(len(values))
}

// calculateStdDev calculates the standard deviation of a slice of float64 values
func calculateStdDev(values []float64, mean float64) float64 {
	if len(values) <= 1 {
		return 0
	}
	
	var sumSquares float64
	for _, v := range values {
		diff := v - mean
		sumSquares += diff * diff
	}
	
	variance := sumSquares / float64(len(values)-1)
	return math.Sqrt(variance)
}

func init() {
	rootCmd.AddCommand(analyzeCmd)
	analyzeCmd.Flags().String("data-file", "", "Path to the JSON monitoring data file (required)")
	analyzeCmd.Flags().String("output", "", "Output format: json (default: human-readable)")
	analyzeCmd.Flags().String("target", "", "Analyze specific target only (optional)")
	analyzeCmd.Flags().Bool("show-trends", false, "Include trend analysis and anomaly detection")
	analyzeCmd.Flags().Bool("compare-periods", false, "Compare performance between different time periods")
	analyzeCmd.MarkFlagRequired("data-file")
}