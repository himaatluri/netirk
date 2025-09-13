package cmd

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/himasagaratluri/netirk/cmd/helpers"
	"github.com/spf13/cobra"
)

// MonitorConfig holds the configuration for monitoring session
type MonitorConfig struct {
	TargetsFile    string
	Interval       time.Duration
	Duration       time.Duration
	OutputFile     string
	OutputFormat   string
	AlertThreshold int
	AlertWebhook   string
	AlertEmail     string
}

var monitorCmd = &cobra.Command{
	Use:   "monitor",
	Short: "Continuously monitor network targets",
	Long: `Monitor network targets continuously at specified intervals.
Collect timing metrics, detect failures, and generate alerts for HTTP, HTTPS, and TCP targets.

The monitor command provides comprehensive network monitoring capabilities including:
• Continuous health checking with configurable intervals
• Response time measurement and SSL certificate monitoring
• Failure detection with customizable alert thresholds
• Multiple output formats (JSON, CSV, HTML, Prometheus)
• Real-time status display and graceful shutdown handling

Configuration File Format:
The targets file should be in YAML format with the following structure:

  targets:
    - url: "https://api.example.com"
      timeout: "30s"
      interval: "60s"
      expected_status: 200
      ssl_check: true
      headers:
        Authorization: "Bearer token"
    - url: "tcp://database.example.com:5432"
      timeout: "10s"
      interval: "30s"

Use 'netirk validate --generate-example' to create a complete example configuration.

Examples:
  # Basic monitoring with 30-second intervals
  netirk monitor --targets-file targets.yaml --interval 30s

  # Monitor for 1 hour and save results to JSON
  netirk monitor --targets-file targets.yaml --duration 1h --output-file monitoring.json

  # Enable alerts after 3 consecutive failures
  netirk monitor --targets-file targets.yaml --alert-threshold 3

  # Monitor with webhook notifications
  netirk monitor --targets-file targets.yaml --alert-webhook https://hooks.slack.com/services/...

  # Export results in multiple formats
  netirk monitor --targets-file targets.yaml --output-file results --output-format html

  # Comprehensive monitoring with all features
  netirk monitor --targets-file targets.yaml --interval 30s --duration 2h \
    --alert-threshold 2 --alert-webhook https://hooks.slack.com/... \
    --output-file monitoring-session --output-format json`,
	Run: func(cmd *cobra.Command, args []string) {
		// Check global flags
		verbose, _ := rootCmd.PersistentFlags().GetBool("verbose")
		quiet, _ := rootCmd.PersistentFlags().GetBool("quiet")
		
		// Display banner unless quiet mode is enabled
		if !quiet {
			helpers.GreetBanner()
		}
		
		// Set log level based on verbose/quiet flags
		if verbose {
			log.SetFlags(log.LstdFlags | log.Lshortfile)
		} else if quiet {
			log.SetOutput(io.Discard)
		}
		
		// Parse command flags into config
		config, err := parseMonitorFlags(cmd)
		if err != nil {
			log.Fatalf("Error parsing flags: %v", err)
		}
		
		// Validate configuration
		if err := validateMonitorConfig(config); err != nil {
			log.Fatalf("Configuration error: %v", err)
		}
		
		// Setup graceful shutdown handling
		setupSignalHandling()
		
		// Start monitoring
		if err := runMonitoring(config); err != nil {
			log.Fatalf("Monitoring failed: %v", err)
		}
	},
}

// parseMonitorFlags extracts and validates command line flags
func parseMonitorFlags(cmd *cobra.Command) (*MonitorConfig, error) {
	config := &MonitorConfig{}
	
	var err error
	
	// Required flags
	config.TargetsFile, _ = cmd.Flags().GetString("targets-file")
	
	// Timing flags
	intervalStr, _ := cmd.Flags().GetString("interval")
	if config.Interval, err = time.ParseDuration(intervalStr); err != nil {
		return nil, fmt.Errorf("invalid interval format: %v", err)
	}
	
	durationStr, _ := cmd.Flags().GetString("duration")
	if durationStr != "" {
		if config.Duration, err = time.ParseDuration(durationStr); err != nil {
			return nil, fmt.Errorf("invalid duration format: %v", err)
		}
	}
	
	// Output flags
	config.OutputFile, _ = cmd.Flags().GetString("output-file")
	config.OutputFormat, _ = cmd.Flags().GetString("output-format")
	
	// Check if global output flag is set and no local output-format is specified
	if config.OutputFormat == "" {
		if globalOutput, _ := rootCmd.Flags().GetString("output"); globalOutput != "" {
			config.OutputFormat = globalOutput
		}
	}
	
	// Alert flags
	config.AlertThreshold, _ = cmd.Flags().GetInt("alert-threshold")
	config.AlertWebhook, _ = cmd.Flags().GetString("alert-webhook")
	config.AlertEmail, _ = cmd.Flags().GetString("alert-email")
	
	return config, nil
}

// validateMonitorConfig validates the monitoring configuration
func validateMonitorConfig(config *MonitorConfig) error {
	// Validate targets file exists
	if config.TargetsFile == "" {
		return fmt.Errorf("targets-file is required")
	}
	
	if _, err := os.Stat(config.TargetsFile); os.IsNotExist(err) {
		return fmt.Errorf("targets file does not exist: %s", config.TargetsFile)
	}
	
	// Validate interval
	if config.Interval <= 0 {
		return fmt.Errorf("interval must be positive")
	}
	
	// Validate duration if specified
	if config.Duration < 0 {
		return fmt.Errorf("duration cannot be negative")
	}
	
	// Validate output format if specified
	if config.OutputFormat != "" {
		validFormats := map[string]bool{
			"json":       true,
			"csv":        true,
			"html":       true,
			"prometheus": true,
		}
		if !validFormats[config.OutputFormat] {
			return fmt.Errorf("invalid output format: %s (valid: json, csv, html, prometheus)", config.OutputFormat)
		}
	}
	
	// Validate alert threshold
	if config.AlertThreshold < 0 {
		return fmt.Errorf("alert threshold cannot be negative")
	}
	
	return nil
}

// Global variables for monitoring control
var (
	monitoringCtx       context.Context
	monitoringCancel    context.CancelFunc
	monitoringWg        sync.WaitGroup
	storage             helpers.DataStorage
	errorHandler        *helpers.ErrorHandler
	recoveryManager     *helpers.RecoveryManager
	performanceManager  *helpers.PerformanceManager
)

// runMonitoring executes the main monitoring loop with comprehensive error handling and performance optimization
func runMonitoring(config *MonitorConfig) error {
	// Initialize error handling system
	errorHandler = helpers.NewErrorHandler(log.Default())
	recoveryManager = helpers.NewRecoveryManager(errorHandler, log.Default())
	
	// Initialize performance manager with optimized settings
	perfConfig := helpers.DefaultPerformanceConfig()
	performanceManager = helpers.NewPerformanceManager(perfConfig, log.Default())
	defer performanceManager.Cleanup()
	
	// Parse target configuration with error handling
	targets, err := helpers.ParseEnhancedTargetFile(config.TargetsFile)
	if err != nil {
		netirkErr := errorHandler.HandleError(err, "parse_targets")
		return fmt.Errorf("failed to parse targets file: %s", netirkErr.GetUserFriendlyMessage())
	}

	if len(targets.Targets) == 0 {
		return helpers.CreateValidationError("configuration", fmt.Errorf("no targets found in configuration file"))
	}

	// Initialize optimized storage with performance management
	monitoringConfig := helpers.MonitoringConfig{
		Targets:  make([]string, len(targets.Targets)),
		Interval: config.Interval,
		Duration: config.Duration,
	}
	
	for i, target := range targets.Targets {
		monitoringConfig.Targets[i] = target.URL
	}
	
	// Use optimized storage for better performance in long-running sessions
	if config.Duration > 30*time.Minute {
		log.Printf("Using optimized storage for long-running session")
		performanceManager.OptimizeForLongRunning()
		storage = helpers.NewOptimizedStorage(monitoringConfig, performanceManager, log.Default())
	} else {
		storage = helpers.NewInMemoryStorage(monitoringConfig)
	}

	// Setup monitoring context
	monitoringCtx, monitoringCancel = context.WithCancel(context.Background())
	defer func() {
		monitoringCancel()
		// Set end time when monitoring completes
		if storage != nil {
			storage.SetEndTime(time.Now())
		}
	}()

	// Setup graceful shutdown
	setupSignalHandling()

	// Start monitoring
	fmt.Printf("Starting monitoring of %d targets with interval: %v\n", len(targets.Targets), config.Interval)
	if config.Duration > 0 {
		fmt.Printf("Monitoring duration: %v\n", config.Duration)
		
		// Set up duration-based cancellation
		go func() {
			time.Sleep(config.Duration)
			fmt.Println("\nMonitoring duration reached. Stopping...")
			monitoringCancel()
		}()
	}

	// Start periodic data backup if output file is specified
	if config.OutputFile != "" {
		monitoringWg.Add(1)
		go periodicDataBackup(config.OutputFile, config.Interval)
	}

	// Start monitoring loop with performance optimization
	monitoringWg.Add(1)
	go monitoringLoop(targets.Targets, config)

	// Start performance monitoring if enabled
	if len(targets.Targets) > 10 {
		monitoringWg.Add(1)
		go performanceMonitoringLoop()
	}

	// Wait for monitoring to complete
	monitoringWg.Wait()

	// Save final data if output file is specified
	if config.OutputFile != "" {
		fmt.Printf("Saving final monitoring data to: %s\n", config.OutputFile)
		fmt.Printf("DEBUG: Output format is: '%s'\n", config.OutputFormat)
		
		// Use specified format or default to JSON
		if config.OutputFormat != "" {
			fmt.Printf("DEBUG: Using exportWithFormat with format: %s\n", config.OutputFormat)
			if err := exportWithFormat(storage.GetSession(), config.OutputFile, config.OutputFormat); err != nil {
				return fmt.Errorf("failed to export monitoring data: %w", err)
			}
		} else {
			fmt.Printf("DEBUG: Using default JSON format\n")
			if err := storage.SaveToFile(config.OutputFile); err != nil {
				return fmt.Errorf("failed to save monitoring data: %w", err)
			}
		}
	}

	// Display summary
	displayMonitoringSummary()

	return nil
}

// monitoringLoop runs the continuous monitoring process
func monitoringLoop(targets []helpers.ParsedTargetConfig, config *MonitorConfig) {
	defer monitoringWg.Done()

	ticker := time.NewTicker(config.Interval)
	defer ticker.Stop()

	// Perform initial collection
	fmt.Println("Performing initial monitoring check...")
	collectAndDisplayMetrics(targets)

	// Continue monitoring at intervals
	for {
		select {
		case <-monitoringCtx.Done():
			fmt.Println("Monitoring stopped.")
			return
		case <-ticker.C:
			collectAndDisplayMetrics(targets)
		}
	}
}

// collectAndDisplayMetrics collects metrics for all targets and displays results with error handling and performance optimization
func collectAndDisplayMetrics(targets []helpers.ParsedTargetConfig) {
	fmt.Printf("\n[%s] Collecting metrics for %d targets...\n", time.Now().Format("15:04:05"), len(targets))
	
	startTime := time.Now()
	
	// Use optimized collection for better performance
	var err error
	if performanceManager != nil && len(targets) > 5 {
		err = helpers.OptimizedCollectMetrics(targets, storage, performanceManager, log.Default())
	} else {
		// Use enhanced collection with error handling and recovery for smaller target sets
		err = helpers.CollectMetricsWithRecovery(targets, storage, errorHandler, recoveryManager)
	}
	collectionTime := time.Since(startTime)

	// Display collection results with user-friendly error messages
	if err != nil {
		userFriendlyError := helpers.FormatErrorForUser(err)
		fmt.Printf("Collection completed with issues in %v: %s\n", collectionTime, userFriendlyError)
		
		// Log detailed error for debugging
		log.Printf("Collection error details: %v", err)
		
		// Display recovery status if in degradation mode
		if recoveryManager.IsInDegradationMode() {
			failedTargets := recoveryManager.GetFailedTargets()
			fmt.Printf("System in degradation mode - %d targets failing: %v\n", len(failedTargets), failedTargets)
		}
	} else {
		fmt.Printf("Collection completed successfully in %v\n", collectionTime)
	}

	// Display current status for each target with recovery information
	displayCurrentStatusWithRecovery(targets)
}

// displayCurrentStatusWithRecovery shows the current status of all targets with recovery information
func displayCurrentStatusWithRecovery(targets []helpers.ParsedTargetConfig) {
	fmt.Println("\nCurrent Status:")
	fmt.Println("Target                                    Status      Response Time    Last Check    Recovery")
	fmt.Println("----------------------------------------  ----------  ---------------  ----------    --------")

	for _, target := range targets {
		data := storage.GetLatestData(target.URL)
		recoveryStatus := recoveryManager.GetTargetStatus(target.URL)
		
		if data != nil {
			status := data.Status
			responseTime := data.ResponseTime.String()
			lastCheck := data.Timestamp.Format("15:04:05")
			
			// Enhance status display with recovery information
			if recoveryStatus != "healthy" {
				status = fmt.Sprintf("%s(%s)", status, recoveryStatus)
			}
			
			// Truncate long URLs for display
			displayURL := target.URL
			if len(displayURL) > 40 {
				displayURL = displayURL[:37] + "..."
			}
			
			// Color-code status for better visibility (if terminal supports it)
			statusDisplay := status
			if data.Status == "success" {
				statusDisplay = status // Keep as is for success
			} else if data.Status == "failed" {
				statusDisplay = status // Keep as is for failures
			}
			
			recoveryDisplay := recoveryStatus
			if recoveryStatus == "healthy" {
				recoveryDisplay = "OK"
			}
			
			fmt.Printf("%-40s  %-10s  %-15s  %-10s    %s\n", 
				displayURL, statusDisplay, responseTime, lastCheck, recoveryDisplay)
		} else {
			displayURL := target.URL
			if len(displayURL) > 40 {
				displayURL = displayURL[:37] + "..."
			}
			fmt.Printf("%-40s  %-10s  %-15s  %-10s    %s\n", 
				displayURL, "pending", "-", "-", "waiting")
		}
	}
	
	// Display recovery statistics if there are any issues
	if recoveryManager.IsInDegradationMode() {
		stats := recoveryManager.GetRecoveryStats()
		fmt.Printf("\nRecovery Status: %d failed targets, %d alerts pending\n", 
			stats["failed_targets_count"], stats["alert_backlog_size"])
	}
}

// saveDataOnShutdown saves monitoring data when the program is interrupted with error handling
func saveDataOnShutdown() error {
	if storage == nil {
		return helpers.CreateStorageError("shutdown_save", fmt.Errorf("no storage available"))
	}

	// Generate a default filename with timestamp if none was specified
	filename := fmt.Sprintf("monitoring-session-%s.json", time.Now().Format("2006-01-02-15-04-05"))
	
	// Check if we have any data to save
	session := storage.GetSession()
	if len(session.Data) == 0 {
		fmt.Println("No monitoring data to save.")
		return nil
	}

	// Save the data with recovery mechanism
	if recoveryManager != nil {
		err := recoveryManager.RecoverStorageOperation(func() error {
			return storage.SaveToFile(filename)
		}, "shutdown_save")
		
		if err != nil {
			userFriendlyError := helpers.FormatErrorForUser(err)
			fmt.Printf("Warning: Failed to save monitoring data: %s\n", userFriendlyError)
			log.Printf("Shutdown save error details: %v", err)
			return err
		}
	} else {
		// Fallback to direct save if recovery manager is not available
		if err := storage.SaveToFile(filename); err != nil {
			storageErr := helpers.CreateStorageError("shutdown_save", err)
			fmt.Printf("Warning: Failed to save monitoring data: %s\n", storageErr.GetUserFriendlyMessage())
			return storageErr
		}
	}

	fmt.Printf("Saved %d monitoring data points to: %s\n", len(session.Data), filename)
	return nil
}

// periodicDataBackup performs periodic backups of monitoring data with error handling
func periodicDataBackup(filename string, interval time.Duration) {
	defer monitoringWg.Done()

	// Calculate backup interval (every 10 monitoring cycles or minimum 5 minutes)
	backupInterval := interval * 10
	if backupInterval < 5*time.Minute {
		backupInterval = 5 * time.Minute
	}

	ticker := time.NewTicker(backupInterval)
	defer ticker.Stop()

	fmt.Printf("Periodic data backup enabled (every %v)\n", backupInterval)

	for {
		select {
		case <-monitoringCtx.Done():
			// Final backup before shutdown with error handling
			if storage != nil && storage.GetBufferSize() > 0 {
				fmt.Println("Performing final data backup...")
				
				if recoveryManager != nil {
					err := recoveryManager.RecoverStorageOperation(func() error {
						return storage.SaveToFile(filename)
					}, "final_backup")
					
					if err != nil {
						fmt.Printf("Warning: Failed to perform final backup: %s\n", helpers.FormatErrorForUser(err))
						log.Printf("Final backup error details: %v", err)
					} else {
						fmt.Printf("Final backup completed successfully\n")
					}
				} else {
					if err := storage.SaveToFile(filename); err != nil {
						fmt.Printf("Warning: Failed to perform final backup: %s\n", helpers.FormatErrorForUser(err))
					}
				}
			}
			return
			
		case <-ticker.C:
			if storage != nil && storage.GetBufferSize() > 0 {
				// Create backup filename with timestamp
				backupFilename := fmt.Sprintf("%s.backup-%s", filename, time.Now().Format("2006-01-02-15-04-05"))
				
				// Perform backup with error handling
				if recoveryManager != nil {
					err := recoveryManager.RecoverStorageOperation(func() error {
						return storage.SaveToFile(backupFilename)
					}, "periodic_backup")
					
					if err != nil {
						fmt.Printf("Warning: Failed to create periodic backup: %s\n", helpers.FormatErrorForUser(err))
						log.Printf("Periodic backup error details: %v", err)
					} else {
						fmt.Printf("Created periodic backup: %s (%d data points)\n", backupFilename, storage.GetBufferSize())
					}
				} else {
					if err := storage.SaveToFile(backupFilename); err != nil {
						fmt.Printf("Warning: Failed to create periodic backup: %s\n", helpers.FormatErrorForUser(err))
					} else {
						fmt.Printf("Created periodic backup: %s (%d data points)\n", backupFilename, storage.GetBufferSize())
					}
				}
			}
		}
	}
}

// displayMonitoringSummary shows a summary of the monitoring session
func displayMonitoringSummary() {
	session := storage.GetSession()
	duration := session.EndTime.Sub(session.StartTime)
	if session.EndTime.IsZero() {
		duration = time.Since(session.StartTime)
	}

	fmt.Printf("\n=== Monitoring Summary ===\n")
	fmt.Printf("Duration: %v\n", duration)
	fmt.Printf("Total data points collected: %d\n", len(session.Data))
	fmt.Printf("Targets monitored: %d\n", len(session.Config.Targets))

	// Calculate success rates per target
	targetStats := make(map[string]struct {
		total   int
		success int
	})

	for _, data := range session.Data {
		stats := targetStats[data.Target]
		stats.total++
		if data.Status == "success" {
			stats.success++
		}
		targetStats[data.Target] = stats
	}

	fmt.Println("\nSuccess Rates:")
	for target, stats := range targetStats {
		successRate := float64(stats.success) / float64(stats.total) * 100
		displayURL := target
		if len(displayURL) > 50 {
			displayURL = displayURL[:47] + "..."
		}
		fmt.Printf("  %-50s: %.1f%% (%d/%d)\n", displayURL, successRate, stats.success, stats.total)
	}
}

// performanceMonitoringLoop monitors and logs performance metrics during monitoring
func performanceMonitoringLoop() {
	defer monitoringWg.Done()

	ticker := time.NewTicker(60 * time.Second) // Log performance stats every minute
	defer ticker.Stop()

	for {
		select {
		case <-monitoringCtx.Done():
			// Log final performance stats
			if performanceManager != nil {
				fmt.Println("\n=== Final Performance Statistics ===")
				performanceManager.LogPerformanceStats()
				
				utilization := performanceManager.GetResourceUtilization()
				fmt.Printf("Resource Utilization Summary:\n")
				for key, value := range utilization {
					fmt.Printf("  %s: %v\n", key, value)
				}
			}
			return
		case <-ticker.C:
			if performanceManager != nil {
				performanceManager.LogPerformanceStats()
				
				// Check memory usage and trigger GC if needed
				withinLimit, memoryUsage := performanceManager.CheckMemoryUsage()
				if !withinLimit {
					fmt.Printf("High memory usage detected (%.2f MB), optimizing...\n", memoryUsage)
				}
			}
		}
	}
}

// exportWithFormat exports monitoring data using the specified format
func exportWithFormat(session *helpers.MonitoringSession, filename string, format string) error {
	exporter, err := helpers.GetExporter(format)
	if err != nil {
		return fmt.Errorf("failed to get exporter: %w", err)
	}
	
	// Add appropriate file extension if not present
	if !strings.Contains(filename, ".") {
		filename += exporter.GetFileExtension()
	}
	
	return helpers.ExportToFile(session, exporter, filename)
}

// setupSignalHandling configures graceful shutdown on Ctrl+C
func setupSignalHandling() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	
	go func() {
		<-c
		fmt.Println("\nReceived interrupt signal. Gracefully shutting down...")
		
		// Save data before shutdown if storage is available
		if storage != nil {
			fmt.Println("Saving monitoring data before shutdown...")
			if err := saveDataOnShutdown(); err != nil {
				fmt.Printf("Warning: Failed to save data on shutdown: %v\n", err)
			} else {
				fmt.Println("Monitoring data saved successfully.")
			}
		}
		
		if monitoringCancel != nil {
			monitoringCancel()
		}
	}()
}

func init() {
	rootCmd.AddCommand(monitorCmd)
	
	// Required flags
	monitorCmd.Flags().String("targets-file", "", "Path to YAML file containing target configurations (required)")
	monitorCmd.MarkFlagRequired("targets-file")
	
	// Timing flags
	monitorCmd.Flags().String("interval", "30s", "Monitoring interval (e.g., 30s, 1m, 5m)")
	monitorCmd.Flags().String("duration", "", "Total monitoring duration (e.g., 1h, 30m). If not specified, runs indefinitely")
	
	// Output flags
	monitorCmd.Flags().String("output-file", "", "File to save monitoring data")
	monitorCmd.Flags().String("output-format", "", "Output format: json, csv, html, prometheus")
	
	// Alert flags
	monitorCmd.Flags().Int("alert-threshold", 0, "Number of consecutive failures before triggering alert")
	monitorCmd.Flags().String("alert-webhook", "", "Webhook URL for alert notifications")
	monitorCmd.Flags().String("alert-email", "", "Email address for alert notifications")
}