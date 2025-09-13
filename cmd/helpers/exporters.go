package helpers

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
)

// Exporter interface defines methods for exporting monitoring data
type Exporter interface {
	Export(session *MonitoringSession, writer io.Writer) error
	GetContentType() string
	GetFileExtension() string
}

// JSONExporter exports monitoring data in JSON format
type JSONExporter struct{}

// NewJSONExporter creates a new JSON exporter
func NewJSONExporter() *JSONExporter {
	return &JSONExporter{}
}

// Export exports monitoring session data as JSON
func (e *JSONExporter) Export(session *MonitoringSession, writer io.Writer) error {
	if session == nil {
		return fmt.Errorf("monitoring session cannot be nil")
	}

	// Create export structure with formatted data
	exportData := struct {
		MonitoringSession struct {
			StartTime string                 `json:"start_time"`
			EndTime   string                 `json:"end_time"`
			Config    MonitoringConfig       `json:"config"`
			Data      []ExportMonitoringData `json:"data"`
		} `json:"monitoring_session"`
	}{}

	exportData.MonitoringSession.StartTime = session.StartTime.Format(time.RFC3339)
	if !session.EndTime.IsZero() {
		exportData.MonitoringSession.EndTime = session.EndTime.Format(time.RFC3339)
	}
	exportData.MonitoringSession.Config = session.Config

	// Convert monitoring data to export format
	exportData.MonitoringSession.Data = make([]ExportMonitoringData, len(session.Data))
	for i, data := range session.Data {
		exportData.MonitoringSession.Data[i] = convertToExportFormat(data)
	}

	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(exportData)
}

// GetContentType returns the MIME type for JSON
func (e *JSONExporter) GetContentType() string {
	return "application/json"
}

// GetFileExtension returns the file extension for JSON files
func (e *JSONExporter) GetFileExtension() string {
	return ".json"
}

// CSVExporter exports monitoring data in CSV format
type CSVExporter struct{}

// NewCSVExporter creates a new CSV exporter
func NewCSVExporter() *CSVExporter {
	return &CSVExporter{}
}

// Export exports monitoring session data as CSV
func (e *CSVExporter) Export(session *MonitoringSession, writer io.Writer) error {
	if session == nil {
		return fmt.Errorf("monitoring session cannot be nil")
	}

	csvWriter := csv.NewWriter(writer)
	defer csvWriter.Flush()

	// Write CSV header
	header := []string{
		"timestamp",
		"target",
		"status",
		"response_time_ms",
		"status_code",
		"error",
		"dns_time_ms",
		"connect_time_ms",
		"tls_time_ms",
		"first_byte_time_ms",
		"ssl_expiry_date",
		"ssl_days_to_expiry",
		"ssl_issuer",
		"ssl_is_valid",
		"ssl_error",
	}

	if err := csvWriter.Write(header); err != nil {
		return fmt.Errorf("failed to write CSV header: %w", err)
	}

	// Write data rows
	for _, data := range session.Data {
		record := e.convertToCSVRecord(data)
		if err := csvWriter.Write(record); err != nil {
			return fmt.Errorf("failed to write CSV record: %w", err)
		}
	}

	return nil
}

// convertToCSVRecord converts MonitoringData to CSV record
func (e *CSVExporter) convertToCSVRecord(data MonitoringData) []string {
	record := make([]string, 15)

	record[0] = data.Timestamp.Format(time.RFC3339)
	record[1] = data.Target
	record[2] = data.Status
	record[3] = strconv.FormatInt(data.ResponseTime.Nanoseconds()/1000000, 10)
	record[4] = strconv.Itoa(data.StatusCode)
	record[5] = data.Error

	// Network trace data
	if data.TraceData != nil {
		record[6] = strconv.FormatInt(data.TraceData.DNSTime.Nanoseconds()/1000000, 10)
		record[7] = strconv.FormatInt(data.TraceData.ConnectTime.Nanoseconds()/1000000, 10)
		record[8] = strconv.FormatInt(data.TraceData.TLSTime.Nanoseconds()/1000000, 10)
		record[9] = strconv.FormatInt(data.TraceData.FirstByteTime.Nanoseconds()/1000000, 10)
	} else {
		record[6] = ""
		record[7] = ""
		record[8] = ""
		record[9] = ""
	}

	// SSL data
	if data.SSLInfo != nil {
		if !data.SSLInfo.ExpiryDate.IsZero() {
			record[10] = data.SSLInfo.ExpiryDate.Format(time.RFC3339)
		} else {
			record[10] = ""
		}
		record[11] = strconv.Itoa(data.SSLInfo.DaysToExpiry)
		record[12] = data.SSLInfo.Issuer
		record[13] = strconv.FormatBool(data.SSLInfo.IsValid)
		record[14] = data.SSLInfo.Error
	} else {
		record[10] = ""
		record[11] = ""
		record[12] = ""
		record[13] = ""
		record[14] = ""
	}

	return record
}

// GetContentType returns the MIME type for CSV
func (e *CSVExporter) GetContentType() string {
	return "text/csv"
}

// GetFileExtension returns the file extension for CSV files
func (e *CSVExporter) GetFileExtension() string {
	return ".csv"
}

// ExportMonitoringData represents monitoring data in export format with millisecond timing
type ExportMonitoringData struct {
	Timestamp      string                `json:"timestamp"`
	Target         string                `json:"target"`
	Status         string                `json:"status"`
	ResponseTimeMs int64                 `json:"response_time_ms"`
	StatusCode     int                   `json:"status_code,omitempty"`
	Error          string                `json:"error,omitempty"`
	SSLInfo        *ExportSSLData        `json:"ssl_info,omitempty"`
	TraceData      *ExportNetworkTrace   `json:"trace_data,omitempty"`
}

// ExportNetworkTrace represents network trace data in export format
type ExportNetworkTrace struct {
	DNSTimeMs       int64 `json:"dns_time_ms"`
	ConnectTimeMs   int64 `json:"connect_time_ms"`
	TLSTimeMs       int64 `json:"tls_time_ms"`
	FirstByteTimeMs int64 `json:"first_byte_time_ms"`
}

// ExportSSLData represents SSL data in export format
type ExportSSLData struct {
	ExpiryDate   string `json:"expiry_date"`
	DaysToExpiry int    `json:"days_to_expiry"`
	Issuer       string `json:"issuer"`
	IsValid      bool   `json:"is_valid"`
	Error        string `json:"error,omitempty"`
}

// convertToExportFormat converts MonitoringData to export format
func convertToExportFormat(data MonitoringData) ExportMonitoringData {
	export := ExportMonitoringData{
		Timestamp:      data.Timestamp.Format(time.RFC3339),
		Target:         data.Target,
		Status:         data.Status,
		ResponseTimeMs: data.ResponseTime.Nanoseconds() / 1000000,
		StatusCode:     data.StatusCode,
		Error:          data.Error,
	}

	// Convert trace data
	if data.TraceData != nil {
		export.TraceData = &ExportNetworkTrace{
			DNSTimeMs:       data.TraceData.DNSTime.Nanoseconds() / 1000000,
			ConnectTimeMs:   data.TraceData.ConnectTime.Nanoseconds() / 1000000,
			TLSTimeMs:       data.TraceData.TLSTime.Nanoseconds() / 1000000,
			FirstByteTimeMs: data.TraceData.FirstByteTime.Nanoseconds() / 1000000,
		}
	}

	// Convert SSL data
	if data.SSLInfo != nil {
		export.SSLInfo = &ExportSSLData{
			DaysToExpiry: data.SSLInfo.DaysToExpiry,
			Issuer:       data.SSLInfo.Issuer,
			IsValid:      data.SSLInfo.IsValid,
			Error:        data.SSLInfo.Error,
		}
		
		if !data.SSLInfo.ExpiryDate.IsZero() {
			export.SSLInfo.ExpiryDate = data.SSLInfo.ExpiryDate.Format(time.RFC3339)
		}
	}

	return export
}

// ExportToFile exports monitoring session data to a file using the specified exporter
func ExportToFile(session *MonitoringSession, exporter Exporter, filename string) error {
	if session == nil {
		return fmt.Errorf("monitoring session cannot be nil")
	}
	if exporter == nil {
		return fmt.Errorf("exporter cannot be nil")
	}
	if strings.TrimSpace(filename) == "" {
		return fmt.Errorf("filename cannot be empty")
	}

	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

	if err := exporter.Export(session, file); err != nil {
		return fmt.Errorf("failed to export data: %w", err)
	}

	return nil
}

// HTMLExporter exports monitoring data as HTML report with charts
type HTMLExporter struct{}

// NewHTMLExporter creates a new HTML exporter
func NewHTMLExporter() *HTMLExporter {
	return &HTMLExporter{}
}

// Export exports monitoring session data as HTML report
func (e *HTMLExporter) Export(session *MonitoringSession, writer io.Writer) error {
	if session == nil {
		return fmt.Errorf("monitoring session cannot be nil")
	}

	// Generate HTML report
	html := e.generateHTMLReport(session)
	_, err := writer.Write([]byte(html))
	return err
}

// GetContentType returns the MIME type for HTML
func (e *HTMLExporter) GetContentType() string {
	return "text/html"
}

// GetFileExtension returns the file extension for HTML files
func (e *HTMLExporter) GetFileExtension() string {
	return ".html"
}

// generateHTMLReport creates the complete HTML report
func (e *HTMLExporter) generateHTMLReport(session *MonitoringSession) string {
	summary := e.generateSummary(session)
	chartData := e.generateChartData(session)
	
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Netirk Monitoring Report</title>
    <script src="https://cdn.jsdelivr.net/npm/chart.js"></script>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            margin: 0;
            padding: 20px;
            background-color: #f5f5f5;
            color: #333;
        }
        .container {
            max-width: 1200px;
            margin: 0 auto;
            background: white;
            border-radius: 8px;
            box-shadow: 0 2px 10px rgba(0,0,0,0.1);
            overflow: hidden;
        }
        .header {
            background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%);
            color: white;
            padding: 30px;
            text-align: center;
        }
        .header h1 {
            margin: 0;
            font-size: 2.5em;
            font-weight: 300;
        }
        .header p {
            margin: 10px 0 0 0;
            opacity: 0.9;
            font-size: 1.1em;
        }
        .content {
            padding: 30px;
        }
        .summary {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));
            gap: 20px;
            margin-bottom: 40px;
        }
        .summary-card {
            background: #f8f9fa;
            padding: 20px;
            border-radius: 8px;
            border-left: 4px solid #667eea;
        }
        .summary-card h3 {
            margin: 0 0 10px 0;
            color: #495057;
            font-size: 0.9em;
            text-transform: uppercase;
            letter-spacing: 0.5px;
        }
        .summary-card .value {
            font-size: 2em;
            font-weight: bold;
            color: #333;
        }
        .summary-card .unit {
            font-size: 0.8em;
            color: #6c757d;
            margin-left: 5px;
        }
        .charts {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(400px, 1fr));
            gap: 30px;
            margin-bottom: 40px;
        }
        .chart-container {
            background: white;
            border: 1px solid #e9ecef;
            border-radius: 8px;
            padding: 20px;
        }
        .chart-container h3 {
            margin: 0 0 20px 0;
            color: #495057;
            text-align: center;
        }
        .data-table {
            overflow-x: auto;
            margin-top: 20px;
        }
        table {
            width: 100%%;
            border-collapse: collapse;
            background: white;
            border-radius: 8px;
            overflow: hidden;
            box-shadow: 0 1px 3px rgba(0,0,0,0.1);
        }
        th, td {
            padding: 12px 15px;
            text-align: left;
            border-bottom: 1px solid #e9ecef;
        }
        th {
            background: #f8f9fa;
            font-weight: 600;
            color: #495057;
            text-transform: uppercase;
            font-size: 0.8em;
            letter-spacing: 0.5px;
        }
        tr:hover {
            background: #f8f9fa;
        }
        .status-success {
            color: #28a745;
            font-weight: bold;
        }
        .status-failure {
            color: #dc3545;
            font-weight: bold;
        }
        .footer {
            text-align: center;
            padding: 20px;
            color: #6c757d;
            font-size: 0.9em;
            border-top: 1px solid #e9ecef;
        }
        .responsive-canvas {
            position: relative;
            height: 300px;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>Netirk Monitoring Report</h1>
            <p>Generated on %s</p>
        </div>
        
        <div class="content">
            %s
            
            <div class="charts">
                <div class="chart-container">
                    <h3>Response Time Trends</h3>
                    <div class="responsive-canvas">
                        <canvas id="responseTimeChart"></canvas>
                    </div>
                </div>
                
                <div class="chart-container">
                    <h3>Success Rate by Target</h3>
                    <div class="responsive-canvas">
                        <canvas id="successRateChart"></canvas>
                    </div>
                </div>
            </div>
            
            <div class="data-table">
                <h3>Detailed Monitoring Data</h3>
                %s
            </div>
        </div>
        
        <div class="footer">
            <p>Report generated by Netirk • %d data points collected</p>
        </div>
    </div>

    <script>
        %s
    </script>
</body>
</html>`, 
		time.Now().Format("January 2, 2006 at 3:04 PM"),
		summary,
		e.generateDataTable(session),
		len(session.Data),
		chartData)
}

// generateSummary creates the summary cards section
func (e *HTMLExporter) generateSummary(session *MonitoringSession) string {
	totalChecks := len(session.Data)
	if totalChecks == 0 {
		return `<div class="summary">
			<div class="summary-card">
				<h3>Total Checks</h3>
				<div class="value">0</div>
			</div>
		</div>`
	}

	// Calculate summary statistics
	successCount := 0
	var totalResponseTime time.Duration
	var minResponseTime, maxResponseTime time.Duration
	targets := make(map[string]bool)
	sslChecks := 0
	sslWarnings := 0

	for i, data := range session.Data {
		if data.Status == "success" {
			successCount++
		}
		
		totalResponseTime += data.ResponseTime
		
		if i == 0 {
			minResponseTime = data.ResponseTime
			maxResponseTime = data.ResponseTime
		} else {
			if data.ResponseTime < minResponseTime {
				minResponseTime = data.ResponseTime
			}
			if data.ResponseTime > maxResponseTime {
				maxResponseTime = data.ResponseTime
			}
		}
		
		targets[data.Target] = true
		
		if data.SSLInfo != nil {
			sslChecks++
			if data.SSLInfo.DaysToExpiry <= 30 && data.SSLInfo.DaysToExpiry > 0 {
				sslWarnings++
			}
		}
	}

	successRate := float64(successCount) / float64(totalChecks) * 100
	avgResponseTime := totalResponseTime / time.Duration(totalChecks)
	duration := session.EndTime.Sub(session.StartTime)
	if session.EndTime.IsZero() {
		duration = time.Since(session.StartTime)
	}

	return fmt.Sprintf(`<div class="summary">
		<div class="summary-card">
			<h3>Total Checks</h3>
			<div class="value">%d</div>
		</div>
		<div class="summary-card">
			<h3>Success Rate</h3>
			<div class="value">%.1f<span class="unit">%%</span></div>
		</div>
		<div class="summary-card">
			<h3>Targets Monitored</h3>
			<div class="value">%d</div>
		</div>
		<div class="summary-card">
			<h3>Average Response Time</h3>
			<div class="value">%d<span class="unit">ms</span></div>
		</div>
		<div class="summary-card">
			<h3>Min Response Time</h3>
			<div class="value">%d<span class="unit">ms</span></div>
		</div>
		<div class="summary-card">
			<h3>Max Response Time</h3>
			<div class="value">%d<span class="unit">ms</span></div>
		</div>
		<div class="summary-card">
			<h3>Monitoring Duration</h3>
			<div class="value">%s</div>
		</div>
		<div class="summary-card">
			<h3>SSL Warnings</h3>
			<div class="value">%d<span class="unit">/ %d</span></div>
		</div>
	</div>`,
		totalChecks,
		successRate,
		len(targets),
		avgResponseTime.Milliseconds(),
		minResponseTime.Milliseconds(),
		maxResponseTime.Milliseconds(),
		e.formatDuration(duration),
		sslWarnings, sslChecks)
}

// generateDataTable creates the detailed data table
func (e *HTMLExporter) generateDataTable(session *MonitoringSession) string {
	if len(session.Data) == 0 {
		return "<p>No monitoring data available.</p>"
	}

	var tableRows strings.Builder
	
	for _, data := range session.Data {
		statusClass := "status-success"
		if data.Status != "success" {
			statusClass = "status-failure"
		}
		
		sslInfo := ""
		if data.SSLInfo != nil {
			if data.SSLInfo.DaysToExpiry > 0 {
				sslInfo = fmt.Sprintf("%d days", data.SSLInfo.DaysToExpiry)
			} else if data.SSLInfo.Error != "" {
				sslInfo = "Error"
			}
		}
		
		errorText := data.Error
		if len(errorText) > 50 {
			errorText = errorText[:47] + "..."
		}
		
		tableRows.WriteString(fmt.Sprintf(`
			<tr>
				<td>%s</td>
				<td>%s</td>
				<td class="%s">%s</td>
				<td>%d ms</td>
				<td>%d</td>
				<td>%s</td>
				<td>%s</td>
			</tr>`,
			data.Timestamp.Format("15:04:05"),
			data.Target,
			statusClass,
			strings.Title(data.Status),
			data.ResponseTime.Milliseconds(),
			data.StatusCode,
			sslInfo,
			errorText))
	}

	return fmt.Sprintf(`
		<table>
			<thead>
				<tr>
					<th>Time</th>
					<th>Target</th>
					<th>Status</th>
					<th>Response Time</th>
					<th>Status Code</th>
					<th>SSL Expiry</th>
					<th>Error</th>
				</tr>
			</thead>
			<tbody>
				%s
			</tbody>
		</table>`, tableRows.String())
}

// generateChartData creates JavaScript code for charts
func (e *HTMLExporter) generateChartData(session *MonitoringSession) string {
	if len(session.Data) == 0 {
		return "// No data available for charts"
	}

	// Prepare data for response time chart
	timeLabels := make([]string, 0)
	responseTimeData := make(map[string][]int64)
	
	// Prepare data for success rate chart
	targetStats := make(map[string]struct {
		total   int
		success int
	})

	for _, data := range session.Data {
		timeLabels = append(timeLabels, data.Timestamp.Format("15:04:05"))
		
		if responseTimeData[data.Target] == nil {
			responseTimeData[data.Target] = make([]int64, 0)
		}
		responseTimeData[data.Target] = append(responseTimeData[data.Target], data.ResponseTime.Milliseconds())
		
		stats := targetStats[data.Target]
		stats.total++
		if data.Status == "success" {
			stats.success++
		}
		targetStats[data.Target] = stats
	}

	// Generate JavaScript for charts
	var jsCode strings.Builder
	
	// Response Time Chart
	jsCode.WriteString(`
		// Response Time Chart
		const responseTimeCtx = document.getElementById('responseTimeChart').getContext('2d');
		const responseTimeChart = new Chart(responseTimeCtx, {
			type: 'line',
			data: {
				labels: [`)
	
	for i, label := range timeLabels {
		if i > 0 {
			jsCode.WriteString(", ")
		}
		jsCode.WriteString(fmt.Sprintf("'%s'", label))
	}
	
	jsCode.WriteString(`],
				datasets: [`)
	
	colors := []string{"#667eea", "#764ba2", "#f093fb", "#f5576c", "#4facfe", "#00f2fe"}
	colorIndex := 0
	
	for target, data := range responseTimeData {
		if colorIndex > 0 {
			jsCode.WriteString(", ")
		}
		
		color := colors[colorIndex%len(colors)]
		jsCode.WriteString(fmt.Sprintf(`{
					label: '%s',
					data: [`, target))
		
		for i, value := range data {
			if i > 0 {
				jsCode.WriteString(", ")
			}
			jsCode.WriteString(fmt.Sprintf("%d", value))
		}
		
		jsCode.WriteString(fmt.Sprintf(`],
					borderColor: '%s',
					backgroundColor: '%s20',
					tension: 0.1
				}`, color, color))
		
		colorIndex++
	}
	
	jsCode.WriteString(`]
			},
			options: {
				responsive: true,
				maintainAspectRatio: false,
				scales: {
					y: {
						beginAtZero: true,
						title: {
							display: true,
							text: 'Response Time (ms)'
						}
					},
					x: {
						title: {
							display: true,
							text: 'Time'
						}
					}
				},
				plugins: {
					legend: {
						position: 'top'
					}
				}
			}
		});`)

	// Success Rate Chart
	jsCode.WriteString(`
		
		// Success Rate Chart
		const successRateCtx = document.getElementById('successRateChart').getContext('2d');
		const successRateChart = new Chart(successRateCtx, {
			type: 'doughnut',
			data: {
				labels: [`)
	
	targetNames := make([]string, 0, len(targetStats))
	successRates := make([]float64, 0, len(targetStats))
	
	for target, stats := range targetStats {
		targetNames = append(targetNames, target)
		successRate := float64(stats.success) / float64(stats.total) * 100
		successRates = append(successRates, successRate)
	}
	
	for i, target := range targetNames {
		if i > 0 {
			jsCode.WriteString(", ")
		}
		jsCode.WriteString(fmt.Sprintf("'%s'", target))
	}
	
	jsCode.WriteString(`],
				datasets: [{
					data: [`)
	
	for i, rate := range successRates {
		if i > 0 {
			jsCode.WriteString(", ")
		}
		jsCode.WriteString(fmt.Sprintf("%.1f", rate))
	}
	
	jsCode.WriteString(`],
					backgroundColor: [`)
	
	for i := range targetNames {
		if i > 0 {
			jsCode.WriteString(", ")
		}
		color := colors[i%len(colors)]
		jsCode.WriteString(fmt.Sprintf("'%s'", color))
	}
	
	jsCode.WriteString(`]
				}]
			},
			options: {
				responsive: true,
				maintainAspectRatio: false,
				plugins: {
					legend: {
						position: 'right'
					},
					tooltip: {
						callbacks: {
							label: function(context) {
								return context.label + ': ' + context.parsed + '%';
							}
						}
					}
				}
			}
		});`)

	return jsCode.String()
}

// formatDuration formats a duration in a human-readable way
func (e *HTMLExporter) formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0fs", d.Seconds())
	} else if d < time.Hour {
		return fmt.Sprintf("%.1fm", d.Minutes())
	} else {
		hours := int(d.Hours())
		minutes := int(d.Minutes()) % 60
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
}

// PrometheusExporter exports monitoring data in Prometheus metrics format
type PrometheusExporter struct{}

// NewPrometheusExporter creates a new Prometheus exporter
func NewPrometheusExporter() *PrometheusExporter {
	return &PrometheusExporter{}
}

// Export exports monitoring session data in Prometheus metrics format
func (e *PrometheusExporter) Export(session *MonitoringSession, writer io.Writer) error {
	if session == nil {
		return fmt.Errorf("monitoring session cannot be nil")
	}

	// Generate Prometheus metrics
	metrics := e.generatePrometheusMetrics(session)
	_, err := writer.Write([]byte(metrics))
	return err
}

// GetContentType returns the MIME type for Prometheus metrics
func (e *PrometheusExporter) GetContentType() string {
	return "text/plain; version=0.0.4"
}

// GetFileExtension returns the file extension for Prometheus metrics files
func (e *PrometheusExporter) GetFileExtension() string {
	return ".prom"
}

// generatePrometheusMetrics creates Prometheus metrics from monitoring data
func (e *PrometheusExporter) generatePrometheusMetrics(session *MonitoringSession) string {
	var metrics strings.Builder
	
	// Add metadata header
	metrics.WriteString("# Netirk Network Monitoring Metrics\n")
	metrics.WriteString("# Generated at: " + time.Now().Format(time.RFC3339) + "\n\n")
	
	// Calculate aggregated metrics per target
	targetMetrics := e.calculateTargetMetrics(session)
	
	// Generate response time metrics
	e.writeResponseTimeMetrics(&metrics, targetMetrics)
	
	// Generate success rate metrics
	e.writeSuccessRateMetrics(&metrics, targetMetrics)
	
	// Generate SSL certificate metrics
	e.writeSSLMetrics(&metrics, session)
	
	// Generate network trace metrics
	e.writeNetworkTraceMetrics(&metrics, targetMetrics)
	
	// Generate status code metrics
	e.writeStatusCodeMetrics(&metrics, targetMetrics)
	
	return metrics.String()
}

// TargetMetrics holds aggregated metrics for a target
type TargetMetrics struct {
	Target           string
	TotalChecks      int
	SuccessfulChecks int
	FailedChecks     int
	AvgResponseTime  float64
	MinResponseTime  float64
	MaxResponseTime  float64
	LastResponseTime float64
	LastStatus       string
	LastTimestamp    time.Time
	StatusCodes      map[int]int
	AvgDNSTime       float64
	AvgConnectTime   float64
	AvgTLSTime       float64
	AvgFirstByteTime float64
}

// calculateTargetMetrics aggregates metrics per target
func (e *PrometheusExporter) calculateTargetMetrics(session *MonitoringSession) map[string]*TargetMetrics {
	targetMetrics := make(map[string]*TargetMetrics)
	
	for _, data := range session.Data {
		target := data.Target
		
		if targetMetrics[target] == nil {
			targetMetrics[target] = &TargetMetrics{
				Target:      target,
				StatusCodes: make(map[int]int),
				MinResponseTime: float64(data.ResponseTime.Milliseconds()),
				MaxResponseTime: float64(data.ResponseTime.Milliseconds()),
			}
		}
		
		tm := targetMetrics[target]
		tm.TotalChecks++
		
		responseTimeMs := float64(data.ResponseTime.Milliseconds())
		tm.AvgResponseTime = (tm.AvgResponseTime*float64(tm.TotalChecks-1) + responseTimeMs) / float64(tm.TotalChecks)
		
		if responseTimeMs < tm.MinResponseTime {
			tm.MinResponseTime = responseTimeMs
		}
		if responseTimeMs > tm.MaxResponseTime {
			tm.MaxResponseTime = responseTimeMs
		}
		
		// Track latest data
		if data.Timestamp.After(tm.LastTimestamp) {
			tm.LastResponseTime = responseTimeMs
			tm.LastStatus = data.Status
			tm.LastTimestamp = data.Timestamp
		}
		
		// Count success/failure
		if data.Status == "success" {
			tm.SuccessfulChecks++
		} else {
			tm.FailedChecks++
		}
		
		// Track status codes
		if data.StatusCode > 0 {
			tm.StatusCodes[data.StatusCode]++
		}
		
		// Aggregate network trace metrics
		if data.TraceData != nil {
			dnsMs := float64(data.TraceData.DNSTime.Milliseconds())
			connectMs := float64(data.TraceData.ConnectTime.Milliseconds())
			tlsMs := float64(data.TraceData.TLSTime.Milliseconds())
			firstByteMs := float64(data.TraceData.FirstByteTime.Milliseconds())
			
			tm.AvgDNSTime = (tm.AvgDNSTime*float64(tm.TotalChecks-1) + dnsMs) / float64(tm.TotalChecks)
			tm.AvgConnectTime = (tm.AvgConnectTime*float64(tm.TotalChecks-1) + connectMs) / float64(tm.TotalChecks)
			tm.AvgTLSTime = (tm.AvgTLSTime*float64(tm.TotalChecks-1) + tlsMs) / float64(tm.TotalChecks)
			tm.AvgFirstByteTime = (tm.AvgFirstByteTime*float64(tm.TotalChecks-1) + firstByteMs) / float64(tm.TotalChecks)
		}
	}
	
	return targetMetrics
}

// writeResponseTimeMetrics writes response time metrics
func (e *PrometheusExporter) writeResponseTimeMetrics(metrics *strings.Builder, targetMetrics map[string]*TargetMetrics) {
	metrics.WriteString("# HELP netirk_response_time_seconds Response time in seconds\n")
	metrics.WriteString("# TYPE netirk_response_time_seconds gauge\n")
	
	for _, tm := range targetMetrics {
		targetLabel := e.sanitizeLabel(tm.Target)
		metrics.WriteString(fmt.Sprintf("netirk_response_time_seconds{target=\"%s\",stat=\"current\"} %.3f %d\n",
			targetLabel, tm.LastResponseTime/1000.0, tm.LastTimestamp.Unix()))
		metrics.WriteString(fmt.Sprintf("netirk_response_time_seconds{target=\"%s\",stat=\"avg\"} %.3f %d\n",
			targetLabel, tm.AvgResponseTime/1000.0, tm.LastTimestamp.Unix()))
		metrics.WriteString(fmt.Sprintf("netirk_response_time_seconds{target=\"%s\",stat=\"min\"} %.3f %d\n",
			targetLabel, tm.MinResponseTime/1000.0, tm.LastTimestamp.Unix()))
		metrics.WriteString(fmt.Sprintf("netirk_response_time_seconds{target=\"%s\",stat=\"max\"} %.3f %d\n",
			targetLabel, tm.MaxResponseTime/1000.0, tm.LastTimestamp.Unix()))
	}
	metrics.WriteString("\n")
}

// writeSuccessRateMetrics writes success rate and check count metrics
func (e *PrometheusExporter) writeSuccessRateMetrics(metrics *strings.Builder, targetMetrics map[string]*TargetMetrics) {
	// Success rate metric
	metrics.WriteString("# HELP netirk_success_rate Success rate as a percentage (0-100)\n")
	metrics.WriteString("# TYPE netirk_success_rate gauge\n")
	
	for _, tm := range targetMetrics {
		targetLabel := e.sanitizeLabel(tm.Target)
		successRate := float64(tm.SuccessfulChecks) / float64(tm.TotalChecks) * 100.0
		metrics.WriteString(fmt.Sprintf("netirk_success_rate{target=\"%s\"} %.2f %d\n",
			targetLabel, successRate, tm.LastTimestamp.Unix()))
	}
	metrics.WriteString("\n")
	
	// Check counts
	metrics.WriteString("# HELP netirk_checks_total Total number of checks performed\n")
	metrics.WriteString("# TYPE netirk_checks_total counter\n")
	
	for _, tm := range targetMetrics {
		targetLabel := e.sanitizeLabel(tm.Target)
		metrics.WriteString(fmt.Sprintf("netirk_checks_total{target=\"%s\",status=\"success\"} %d %d\n",
			targetLabel, tm.SuccessfulChecks, tm.LastTimestamp.Unix()))
		metrics.WriteString(fmt.Sprintf("netirk_checks_total{target=\"%s\",status=\"failure\"} %d %d\n",
			targetLabel, tm.FailedChecks, tm.LastTimestamp.Unix()))
	}
	metrics.WriteString("\n")
	
	// Current status (1 for up, 0 for down)
	metrics.WriteString("# HELP netirk_up Current status of target (1=up, 0=down)\n")
	metrics.WriteString("# TYPE netirk_up gauge\n")
	
	for _, tm := range targetMetrics {
		targetLabel := e.sanitizeLabel(tm.Target)
		upValue := 0
		if tm.LastStatus == "success" {
			upValue = 1
		}
		metrics.WriteString(fmt.Sprintf("netirk_up{target=\"%s\"} %d %d\n",
			targetLabel, upValue, tm.LastTimestamp.Unix()))
	}
	metrics.WriteString("\n")
}

// writeSSLMetrics writes SSL certificate metrics
func (e *PrometheusExporter) writeSSLMetrics(metrics *strings.Builder, session *MonitoringSession) {
	// Find latest SSL data for each target
	latestSSL := make(map[string]*SSLData)
	latestTimestamp := make(map[string]time.Time)
	
	for _, data := range session.Data {
		if data.SSLInfo != nil {
			if latestTimestamp[data.Target].Before(data.Timestamp) {
				latestSSL[data.Target] = data.SSLInfo
				latestTimestamp[data.Target] = data.Timestamp
			}
		}
	}
	
	if len(latestSSL) > 0 {
		// SSL certificate expiry days
		metrics.WriteString("# HELP netirk_ssl_cert_expiry_days Days until SSL certificate expires\n")
		metrics.WriteString("# TYPE netirk_ssl_cert_expiry_days gauge\n")
		
		for target, sslData := range latestSSL {
			targetLabel := e.sanitizeLabel(target)
			metrics.WriteString(fmt.Sprintf("netirk_ssl_cert_expiry_days{target=\"%s\",issuer=\"%s\"} %d %d\n",
				targetLabel, e.sanitizeLabel(sslData.Issuer), sslData.DaysToExpiry, latestTimestamp[target].Unix()))
		}
		metrics.WriteString("\n")
		
		// SSL certificate validity
		metrics.WriteString("# HELP netirk_ssl_cert_valid SSL certificate validity (1=valid, 0=invalid)\n")
		metrics.WriteString("# TYPE netirk_ssl_cert_valid gauge\n")
		
		for target, sslData := range latestSSL {
			targetLabel := e.sanitizeLabel(target)
			validValue := 0
			if sslData.IsValid {
				validValue = 1
			}
			metrics.WriteString(fmt.Sprintf("netirk_ssl_cert_valid{target=\"%s\",issuer=\"%s\"} %d %d\n",
				targetLabel, e.sanitizeLabel(sslData.Issuer), validValue, latestTimestamp[target].Unix()))
		}
		metrics.WriteString("\n")
	}
}

// writeNetworkTraceMetrics writes network timing breakdown metrics
func (e *PrometheusExporter) writeNetworkTraceMetrics(metrics *strings.Builder, targetMetrics map[string]*TargetMetrics) {
	hasTraceData := false
	for _, tm := range targetMetrics {
		if tm.AvgDNSTime > 0 || tm.AvgConnectTime > 0 || tm.AvgTLSTime > 0 || tm.AvgFirstByteTime > 0 {
			hasTraceData = true
			break
		}
	}
	
	if !hasTraceData {
		return
	}
	
	// DNS resolution time
	metrics.WriteString("# HELP netirk_dns_duration_seconds Average DNS resolution time in seconds\n")
	metrics.WriteString("# TYPE netirk_dns_duration_seconds gauge\n")
	
	for _, tm := range targetMetrics {
		if tm.AvgDNSTime > 0 {
			targetLabel := e.sanitizeLabel(tm.Target)
			metrics.WriteString(fmt.Sprintf("netirk_dns_duration_seconds{target=\"%s\"} %.3f %d\n",
				targetLabel, tm.AvgDNSTime/1000.0, tm.LastTimestamp.Unix()))
		}
	}
	metrics.WriteString("\n")
	
	// Connection establishment time
	metrics.WriteString("# HELP netirk_connect_duration_seconds Average connection establishment time in seconds\n")
	metrics.WriteString("# TYPE netirk_connect_duration_seconds gauge\n")
	
	for _, tm := range targetMetrics {
		if tm.AvgConnectTime > 0 {
			targetLabel := e.sanitizeLabel(tm.Target)
			metrics.WriteString(fmt.Sprintf("netirk_connect_duration_seconds{target=\"%s\"} %.3f %d\n",
				targetLabel, tm.AvgConnectTime/1000.0, tm.LastTimestamp.Unix()))
		}
	}
	metrics.WriteString("\n")
	
	// TLS handshake time
	metrics.WriteString("# HELP netirk_tls_duration_seconds Average TLS handshake time in seconds\n")
	metrics.WriteString("# TYPE netirk_tls_duration_seconds gauge\n")
	
	for _, tm := range targetMetrics {
		if tm.AvgTLSTime > 0 {
			targetLabel := e.sanitizeLabel(tm.Target)
			metrics.WriteString(fmt.Sprintf("netirk_tls_duration_seconds{target=\"%s\"} %.3f %d\n",
				targetLabel, tm.AvgTLSTime/1000.0, tm.LastTimestamp.Unix()))
		}
	}
	metrics.WriteString("\n")
	
	// First byte time
	metrics.WriteString("# HELP netirk_first_byte_duration_seconds Average time to first byte in seconds\n")
	metrics.WriteString("# TYPE netirk_first_byte_duration_seconds gauge\n")
	
	for _, tm := range targetMetrics {
		if tm.AvgFirstByteTime > 0 {
			targetLabel := e.sanitizeLabel(tm.Target)
			metrics.WriteString(fmt.Sprintf("netirk_first_byte_duration_seconds{target=\"%s\"} %.3f %d\n",
				targetLabel, tm.AvgFirstByteTime/1000.0, tm.LastTimestamp.Unix()))
		}
	}
	metrics.WriteString("\n")
}

// writeStatusCodeMetrics writes HTTP status code metrics
func (e *PrometheusExporter) writeStatusCodeMetrics(metrics *strings.Builder, targetMetrics map[string]*TargetMetrics) {
	hasStatusCodes := false
	for _, tm := range targetMetrics {
		if len(tm.StatusCodes) > 0 {
			hasStatusCodes = true
			break
		}
	}
	
	if !hasStatusCodes {
		return
	}
	
	metrics.WriteString("# HELP netirk_http_status_code_total Total count of HTTP status codes\n")
	metrics.WriteString("# TYPE netirk_http_status_code_total counter\n")
	
	for _, tm := range targetMetrics {
		targetLabel := e.sanitizeLabel(tm.Target)
		for statusCode, count := range tm.StatusCodes {
			metrics.WriteString(fmt.Sprintf("netirk_http_status_code_total{target=\"%s\",code=\"%d\"} %d %d\n",
				targetLabel, statusCode, count, tm.LastTimestamp.Unix()))
		}
	}
	metrics.WriteString("\n")
}

// sanitizeLabel sanitizes a string for use as a Prometheus label value
func (e *PrometheusExporter) sanitizeLabel(label string) string {
	// Replace problematic characters with underscores
	sanitized := strings.ReplaceAll(label, "\"", "_")
	sanitized = strings.ReplaceAll(sanitized, "\\", "_")
	sanitized = strings.ReplaceAll(sanitized, "\n", "_")
	sanitized = strings.ReplaceAll(sanitized, "\r", "_")
	sanitized = strings.ReplaceAll(sanitized, "'", "_")
	return sanitized
}

// GetExporter returns an exporter instance based on the format string
func GetExporter(format string) (Exporter, error) {
	switch strings.ToLower(format) {
	case "json":
		return NewJSONExporter(), nil
	case "csv":
		return NewCSVExporter(), nil
	case "html":
		return NewHTMLExporter(), nil
	case "prometheus":
		return NewPrometheusExporter(), nil
	default:
		return nil, fmt.Errorf("unsupported export format: %s", format)
	}
}