package helpers

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"net/smtp"
	"strings"
	"sync"
	"time"
)

// AlertConfig defines the configuration for alerting system
type AlertConfig struct {
	Enabled           bool        `yaml:"enabled"`
	FailureThreshold  int         `yaml:"failure_threshold"`
	WebhookURL        string      `yaml:"webhook_url"`
	EmailRecipients   []string    `yaml:"email_recipients"`
	EmailConfig       EmailConfig `yaml:"email_config"`
	SSLWarningDays    int         `yaml:"ssl_warning_days"`
	SSLCriticalDays   int         `yaml:"ssl_critical_days"`
}

// EmailConfig defines SMTP configuration for email notifications
type EmailConfig struct {
	SMTPHost     string `yaml:"smtp_host"`
	SMTPPort     int    `yaml:"smtp_port"`
	Username     string `yaml:"username"`
	Password     string `yaml:"password"`
	FromAddress  string `yaml:"from_address"`
	FromName     string `yaml:"from_name"`
	UseTLS       bool   `yaml:"use_tls"`
	UseStartTLS  bool   `yaml:"use_starttls"`
	InsecureTLS  bool   `yaml:"insecure_tls"`
}

// AlertType represents the type of alert
type AlertType string

const (
	AlertTypeFailure  AlertType = "failure"
	AlertTypeRecovery AlertType = "recovery"
	AlertTypeSSL      AlertType = "ssl"
)

// AlertSeverity represents the severity level of an alert
type AlertSeverity string

const (
	SeverityWarning  AlertSeverity = "warning"
	SeverityCritical AlertSeverity = "critical"
)

// Alert represents an alert event
type Alert struct {
	Type        AlertType     `json:"type"`
	Severity    AlertSeverity `json:"severity"`
	Target      string        `json:"target"`
	Message     string        `json:"message"`
	Timestamp   time.Time     `json:"timestamp"`
	FailureCount int          `json:"failure_count,omitempty"`
	Details     string        `json:"details,omitempty"`
}

// AlertManager manages alert state and notifications
type AlertManager struct {
	config        AlertConfig
	failureCounts map[string]int
	lastAlerts    map[string]time.Time
	alertStates   map[string]bool // tracks if target is currently in alert state
	mutex         sync.RWMutex
}

// NewAlertManager creates a new AlertManager instance
func NewAlertManager(config AlertConfig) *AlertManager {
	return &AlertManager{
		config:        config,
		failureCounts: make(map[string]int),
		lastAlerts:    make(map[string]time.Time),
		alertStates:   make(map[string]bool),
	}
}

// ProcessFailure processes a target failure and determines if an alert should be triggered
func (am *AlertManager) ProcessFailure(target string, errorMsg string) *Alert {
	if !am.config.Enabled {
		return nil
	}

	am.mutex.Lock()
	defer am.mutex.Unlock()

	// Increment failure count
	am.failureCounts[target]++
	currentCount := am.failureCounts[target]

	// Check if we've reached the failure threshold
	if currentCount >= am.config.FailureThreshold {
		// Check if we're already in alert state to prevent duplicate notifications
		if !am.alertStates[target] {
			am.alertStates[target] = true
			am.lastAlerts[target] = time.Now()

			alert := &Alert{
				Type:         AlertTypeFailure,
				Severity:     SeverityCritical,
				Target:       target,
				Message:      fmt.Sprintf("Target %s has failed %d consecutive times", target, currentCount),
				Timestamp:    time.Now(),
				FailureCount: currentCount,
				Details:      errorMsg,
			}

			log.Printf("ALERT: %s - %s", alert.Severity, alert.Message)
			return alert
		}
	}

	return nil
}

// ProcessSuccess processes a target success and determines if a recovery alert should be sent
func (am *AlertManager) ProcessSuccess(target string) *Alert {
	if !am.config.Enabled {
		return nil
	}

	am.mutex.Lock()
	defer am.mutex.Unlock()

	// Check if target was previously in alert state
	wasInAlert := am.alertStates[target]

	// Reset failure count and alert state
	am.failureCounts[target] = 0
	am.alertStates[target] = false

	// Send recovery notification if target was previously failing
	if wasInAlert {
		alert := &Alert{
			Type:      AlertTypeRecovery,
			Severity:  SeverityWarning,
			Target:    target,
			Message:   fmt.Sprintf("Target %s has recovered", target),
			Timestamp: time.Now(),
		}

		log.Printf("RECOVERY: %s", alert.Message)
		return alert
	}

	return nil
}

// ProcessSSLAlert processes SSL certificate expiration alerts
func (am *AlertManager) ProcessSSLAlert(target string, daysToExpiry int, certDetails string) *Alert {
	if !am.config.Enabled {
		return nil
	}

	am.mutex.RLock()
	defer am.mutex.RUnlock()

	var alert *Alert
	alertKey := fmt.Sprintf("%s_ssl", target)

	// Check for critical SSL alert (within critical days)
	if daysToExpiry <= am.config.SSLCriticalDays {
		// Prevent duplicate critical alerts within 24 hours
		if lastAlert, exists := am.lastAlerts[alertKey]; !exists || time.Since(lastAlert) > 24*time.Hour {
			am.lastAlerts[alertKey] = time.Now()
			alert = &Alert{
				Type:      AlertTypeSSL,
				Severity:  SeverityCritical,
				Target:    target,
				Message:   fmt.Sprintf("SSL certificate for %s expires in %d days", target, daysToExpiry),
				Timestamp: time.Now(),
				Details:   certDetails,
			}
		}
	} else if daysToExpiry <= am.config.SSLWarningDays {
		// Check for warning SSL alert (within warning days)
		// Prevent duplicate warning alerts within 7 days
		if lastAlert, exists := am.lastAlerts[alertKey]; !exists || time.Since(lastAlert) > 7*24*time.Hour {
			am.lastAlerts[alertKey] = time.Now()
			alert = &Alert{
				Type:      AlertTypeSSL,
				Severity:  SeverityWarning,
				Target:    target,
				Message:   fmt.Sprintf("SSL certificate for %s expires in %d days", target, daysToExpiry),
				Timestamp: time.Now(),
				Details:   certDetails,
			}
		}
	}

	if alert != nil {
		log.Printf("SSL ALERT: %s - %s", alert.Severity, alert.Message)
	}

	return alert
}

// GetFailureCount returns the current failure count for a target
func (am *AlertManager) GetFailureCount(target string) int {
	am.mutex.RLock()
	defer am.mutex.RUnlock()
	return am.failureCounts[target]
}

// IsInAlertState returns whether a target is currently in alert state
func (am *AlertManager) IsInAlertState(target string) bool {
	am.mutex.RLock()
	defer am.mutex.RUnlock()
	return am.alertStates[target]
}

// GetAlertStats returns statistics about current alert states
func (am *AlertManager) GetAlertStats() map[string]interface{} {
	am.mutex.RLock()
	defer am.mutex.RUnlock()

	stats := make(map[string]interface{})
	stats["enabled"] = am.config.Enabled
	stats["failure_threshold"] = am.config.FailureThreshold
	stats["targets_in_alert"] = len(am.alertStates)
	
	failingTargets := make([]string, 0)
	for target, inAlert := range am.alertStates {
		if inAlert {
			failingTargets = append(failingTargets, target)
		}
	}
	stats["failing_targets"] = failingTargets

	return stats
}

// Reset clears all alert state (useful for testing or manual reset)
func (am *AlertManager) Reset() {
	am.mutex.Lock()
	defer am.mutex.Unlock()

	am.failureCounts = make(map[string]int)
	am.lastAlerts = make(map[string]time.Time)
	am.alertStates = make(map[string]bool)
}

// WebhookPayload represents the JSON payload sent to webhook endpoints
type WebhookPayload struct {
	Alert       Alert  `json:"alert"`
	Source      string `json:"source"`
	Environment string `json:"environment,omitempty"`
	Version     string `json:"version,omitempty"`
}

// WebhookNotifier handles webhook notifications for alerts
type WebhookNotifier struct {
	webhookURL string
	client     *http.Client
	maxRetries int
	baseDelay  time.Duration
}

// NewWebhookNotifier creates a new WebhookNotifier instance
func NewWebhookNotifier(webhookURL string) *WebhookNotifier {
	return &WebhookNotifier{
		webhookURL: webhookURL,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		maxRetries: 3,
		baseDelay:  1 * time.Second,
	}
}

// SendAlert sends an alert to the configured webhook endpoint with enhanced error handling
func (wn *WebhookNotifier) SendAlert(alert *Alert) error {
	if wn.webhookURL == "" {
		return CreateAlertError("webhook", fmt.Errorf("webhook URL not configured"))
	}

	if alert == nil {
		return CreateAlertError("webhook", fmt.Errorf("alert cannot be nil"))
	}

	payload := WebhookPayload{
		Alert:  *alert,
		Source: "netirk",
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return CreateAlertError("webhook", fmt.Errorf("failed to marshal webhook payload: %w", err))
	}

	var lastErr error
	for attempt := 0; attempt <= wn.maxRetries; attempt++ {
		if attempt > 0 {
			// Calculate exponential backoff delay
			delay := time.Duration(float64(wn.baseDelay) * math.Pow(2, float64(attempt-1)))
			log.Printf("Webhook delivery failed, retrying in %v (attempt %d/%d)", delay, attempt, wn.maxRetries)
			time.Sleep(delay)
		}

		// Create request with timeout context
		ctx, cancel := context.WithTimeout(context.Background(), wn.client.Timeout)
		req, err := http.NewRequestWithContext(ctx, "POST", wn.webhookURL, bytes.NewBuffer(jsonData))
		cancel()
		
		if err != nil {
			lastErr = CreateAlertError("webhook", fmt.Errorf("failed to create webhook request: %w", err))
			continue
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "netirk-webhook-notifier")

		resp, err := wn.client.Do(req)
		if err != nil {
			lastErr = CreateAlertError("webhook", fmt.Errorf("failed to send webhook request: %w", err))
			continue
		}

		// Check if the response indicates success
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			resp.Body.Close()
			log.Printf("Webhook alert sent successfully to %s (status: %d)", wn.webhookURL, resp.StatusCode)
			return nil
		}

		resp.Body.Close()
		lastErr = CreateAlertError("webhook", fmt.Errorf("webhook returned status %d", resp.StatusCode))

		// Don't retry on client errors (4xx), only on server errors (5xx) and network issues
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			log.Printf("Webhook returned client error %d, not retrying", resp.StatusCode)
			break
		}
	}

	return fmt.Errorf("webhook delivery failed after %d attempts: %w", wn.maxRetries+1, lastErr)
}

// SendAlertAsync sends an alert asynchronously without blocking
func (wn *WebhookNotifier) SendAlertAsync(alert *Alert) {
	go func() {
		if err := wn.SendAlert(alert); err != nil {
			log.Printf("Failed to send webhook alert: %v", err)
		}
	}()
}

// SetRetryConfig allows customization of retry behavior
func (wn *WebhookNotifier) SetRetryConfig(maxRetries int, baseDelay time.Duration) {
	wn.maxRetries = maxRetries
	wn.baseDelay = baseDelay
}

// SetTimeout allows customization of HTTP client timeout
func (wn *WebhookNotifier) SetTimeout(timeout time.Duration) {
	wn.client.Timeout = timeout
}

// EmailNotifier handles email notifications for alerts
type EmailNotifier struct {
	config     EmailConfig
	maxRetries int
	baseDelay  time.Duration
}

// NewEmailNotifier creates a new EmailNotifier instance
func NewEmailNotifier(config EmailConfig) *EmailNotifier {
	return &EmailNotifier{
		config:     config,
		maxRetries: 3,
		baseDelay:  1 * time.Second,
	}
}

// ValidateConfig validates the email configuration
func (en *EmailNotifier) ValidateConfig() error {
	if en.config.SMTPHost == "" {
		return fmt.Errorf("SMTP host is required")
	}
	if en.config.SMTPPort <= 0 || en.config.SMTPPort > 65535 {
		return fmt.Errorf("SMTP port must be between 1 and 65535")
	}
	if en.config.FromAddress == "" {
		return fmt.Errorf("from address is required")
	}
	if !strings.Contains(en.config.FromAddress, "@") {
		return fmt.Errorf("from address must be a valid email address")
	}
	return nil
}

// formatAlertEmail creates the email content for an alert
func (en *EmailNotifier) formatAlertEmail(alert *Alert, recipients []string) (subject string, body string, err error) {
	if alert == nil {
		return "", "", fmt.Errorf("alert cannot be nil")
	}

	// Create subject line based on alert type and severity
	var subjectPrefix string
	switch alert.Type {
	case AlertTypeFailure:
		if alert.Severity == SeverityCritical {
			subjectPrefix = "[CRITICAL]"
		} else {
			subjectPrefix = "[ALERT]"
		}
	case AlertTypeRecovery:
		subjectPrefix = "[RECOVERY]"
	case AlertTypeSSL:
		if alert.Severity == SeverityCritical {
			subjectPrefix = "[SSL CRITICAL]"
		} else {
			subjectPrefix = "[SSL WARNING]"
		}
	default:
		subjectPrefix = "[ALERT]"
	}

	subject = fmt.Sprintf("%s Netirk Alert - %s", subjectPrefix, alert.Target)

	// Create email body
	var bodyBuilder strings.Builder
	bodyBuilder.WriteString("Netirk Network Monitoring Alert\n")
	bodyBuilder.WriteString("================================\n\n")
	
	bodyBuilder.WriteString(fmt.Sprintf("Alert Type: %s\n", strings.ToUpper(string(alert.Type))))
	bodyBuilder.WriteString(fmt.Sprintf("Severity: %s\n", strings.ToUpper(string(alert.Severity))))
	bodyBuilder.WriteString(fmt.Sprintf("Target: %s\n", alert.Target))
	bodyBuilder.WriteString(fmt.Sprintf("Message: %s\n", alert.Message))
	bodyBuilder.WriteString(fmt.Sprintf("Timestamp: %s\n", alert.Timestamp.Format("2006-01-02 15:04:05 UTC")))

	if alert.FailureCount > 0 {
		bodyBuilder.WriteString(fmt.Sprintf("Failure Count: %d\n", alert.FailureCount))
	}

	if alert.Details != "" {
		bodyBuilder.WriteString(fmt.Sprintf("Details: %s\n", alert.Details))
	}

	bodyBuilder.WriteString("\n")
	bodyBuilder.WriteString("This alert was generated by Netirk network monitoring.\n")
	bodyBuilder.WriteString("Please investigate the issue and take appropriate action.\n")

	// Add recovery-specific message
	if alert.Type == AlertTypeRecovery {
		bodyBuilder.WriteString("\nThe target has recovered and is now responding normally.\n")
	}

	// Add SSL-specific guidance
	if alert.Type == AlertTypeSSL {
		bodyBuilder.WriteString("\nSSL Certificate Action Required:\n")
		if alert.Severity == SeverityCritical {
			bodyBuilder.WriteString("- Certificate expires very soon - immediate action required\n")
		} else {
			bodyBuilder.WriteString("- Certificate expires soon - plan renewal\n")
		}
		bodyBuilder.WriteString("- Check certificate validity and renewal process\n")
		bodyBuilder.WriteString("- Update certificate before expiration to avoid service disruption\n")
	}

	return subject, bodyBuilder.String(), nil
}

// SendAlert sends an alert via email with retry logic
func (en *EmailNotifier) SendAlert(alert *Alert, recipients []string) error {
	if err := en.ValidateConfig(); err != nil {
		return fmt.Errorf("email configuration invalid: %w", err)
	}

	if len(recipients) == 0 {
		return fmt.Errorf("no email recipients specified")
	}

	// Validate recipient email addresses
	for _, recipient := range recipients {
		if !strings.Contains(recipient, "@") {
			return fmt.Errorf("invalid email address: %s", recipient)
		}
	}

	subject, body, err := en.formatAlertEmail(alert, recipients)
	if err != nil {
		return fmt.Errorf("failed to format email: %w", err)
	}

	var lastErr error
	for attempt := 0; attempt <= en.maxRetries; attempt++ {
		if attempt > 0 {
			// Calculate exponential backoff delay
			delay := time.Duration(float64(en.baseDelay) * math.Pow(2, float64(attempt-1)))
			log.Printf("Email delivery failed, retrying in %v (attempt %d/%d)", delay, attempt, en.maxRetries)
			time.Sleep(delay)
		}

		err := en.sendEmail(subject, body, recipients)
		if err == nil {
			log.Printf("Email alert sent successfully to %v", recipients)
			return nil
		}

		lastErr = err
		log.Printf("Email delivery attempt %d failed: %v", attempt+1, err)
	}

	return fmt.Errorf("email delivery failed after %d attempts: %w", en.maxRetries+1, lastErr)
}

// sendEmail performs the actual SMTP email sending
func (en *EmailNotifier) sendEmail(subject, body string, recipients []string) error {
	// Create SMTP address
	smtpAddr := fmt.Sprintf("%s:%d", en.config.SMTPHost, en.config.SMTPPort)

	// Prepare email headers and content
	fromName := en.config.FromName
	if fromName == "" {
		fromName = "Netirk Monitor"
	}

	from := fmt.Sprintf("%s <%s>", fromName, en.config.FromAddress)
	
	// Build email message
	var msg strings.Builder
	msg.WriteString(fmt.Sprintf("From: %s\r\n", from))
	msg.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(recipients, ", ")))
	msg.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	msg.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	msg.WriteString("\r\n")
	msg.WriteString(body)

	// Handle different TLS configurations
	if en.config.UseTLS {
		// Direct TLS connection (typically port 465)
		return en.sendEmailWithTLS(smtpAddr, msg.String(), recipients)
	} else if en.config.UseStartTLS {
		// STARTTLS connection (typically port 587)
		return en.sendEmailWithStartTLS(smtpAddr, msg.String(), recipients)
	} else {
		// Plain connection (typically port 25, not recommended for production)
		return en.sendEmailPlain(smtpAddr, msg.String(), recipients)
	}
}

// sendEmailWithTLS sends email using direct TLS connection
func (en *EmailNotifier) sendEmailWithTLS(smtpAddr, message string, recipients []string) error {
	tlsConfig := &tls.Config{
		ServerName:         en.config.SMTPHost,
		InsecureSkipVerify: en.config.InsecureTLS,
	}

	conn, err := tls.Dial("tcp", smtpAddr, tlsConfig)
	if err != nil {
		return fmt.Errorf("failed to establish TLS connection: %w", err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, en.config.SMTPHost)
	if err != nil {
		return fmt.Errorf("failed to create SMTP client: %w", err)
	}
	defer client.Quit()

	return en.authenticateAndSend(client, message, recipients)
}

// sendEmailWithStartTLS sends email using STARTTLS
func (en *EmailNotifier) sendEmailWithStartTLS(smtpAddr, message string, recipients []string) error {
	client, err := smtp.Dial(smtpAddr)
	if err != nil {
		return fmt.Errorf("failed to connect to SMTP server: %w", err)
	}
	defer client.Quit()

	// Start TLS if supported
	if ok, _ := client.Extension("STARTTLS"); ok {
		tlsConfig := &tls.Config{
			ServerName:         en.config.SMTPHost,
			InsecureSkipVerify: en.config.InsecureTLS,
		}
		if err := client.StartTLS(tlsConfig); err != nil {
			return fmt.Errorf("failed to start TLS: %w", err)
		}
	}

	return en.authenticateAndSend(client, message, recipients)
}

// sendEmailPlain sends email using plain connection (not recommended)
func (en *EmailNotifier) sendEmailPlain(smtpAddr, message string, recipients []string) error {
	client, err := smtp.Dial(smtpAddr)
	if err != nil {
		return fmt.Errorf("failed to connect to SMTP server: %w", err)
	}
	defer client.Quit()

	return en.authenticateAndSend(client, message, recipients)
}

// authenticateAndSend handles SMTP authentication and message sending
func (en *EmailNotifier) authenticateAndSend(client *smtp.Client, message string, recipients []string) error {
	// Authenticate if credentials are provided
	if en.config.Username != "" && en.config.Password != "" {
		auth := smtp.PlainAuth("", en.config.Username, en.config.Password, en.config.SMTPHost)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("SMTP authentication failed: %w", err)
		}
	}

	// Set sender
	if err := client.Mail(en.config.FromAddress); err != nil {
		return fmt.Errorf("failed to set sender: %w", err)
	}

	// Set recipients
	for _, recipient := range recipients {
		if err := client.Rcpt(recipient); err != nil {
			return fmt.Errorf("failed to set recipient %s: %w", recipient, err)
		}
	}

	// Send message
	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("failed to get data writer: %w", err)
	}
	defer writer.Close()

	if _, err := writer.Write([]byte(message)); err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}

	return nil
}

// SendAlertAsync sends an alert asynchronously without blocking
func (en *EmailNotifier) SendAlertAsync(alert *Alert, recipients []string) {
	go func() {
		if err := en.SendAlert(alert, recipients); err != nil {
			log.Printf("Failed to send email alert: %v", err)
		}
	}()
}

// SetRetryConfig allows customization of retry behavior
func (en *EmailNotifier) SetRetryConfig(maxRetries int, baseDelay time.Duration) {
	en.maxRetries = maxRetries
	en.baseDelay = baseDelay
}

// NotifyAlert is a convenience method that sends alerts via webhook and email if configured
func (am *AlertManager) NotifyAlert(alert *Alert) {
	if alert == nil {
		return
	}

	// Send webhook notification if configured
	if am.config.WebhookURL != "" {
		notifier := NewWebhookNotifier(am.config.WebhookURL)
		notifier.SendAlertAsync(alert)
	}

	// Send email notification if configured
	if len(am.config.EmailRecipients) > 0 {
		emailNotifier := NewEmailNotifier(am.config.EmailConfig)
		emailNotifier.SendAlertAsync(alert, am.config.EmailRecipients)
	}
}