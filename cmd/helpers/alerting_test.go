package helpers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewAlertManager(t *testing.T) {
	config := AlertConfig{
		Enabled:          true,
		FailureThreshold: 3,
		SSLWarningDays:   30,
		SSLCriticalDays:  7,
	}

	am := NewAlertManager(config)

	if am.config.Enabled != true {
		t.Errorf("Expected Enabled to be true, got %v", am.config.Enabled)
	}
	if am.config.FailureThreshold != 3 {
		t.Errorf("Expected FailureThreshold to be 3, got %d", am.config.FailureThreshold)
	}
	if am.failureCounts == nil {
		t.Error("Expected failureCounts to be initialized")
	}
	if am.lastAlerts == nil {
		t.Error("Expected lastAlerts to be initialized")
	}
	if am.alertStates == nil {
		t.Error("Expected alertStates to be initialized")
	}
}

func TestProcessFailure_BelowThreshold(t *testing.T) {
	config := AlertConfig{
		Enabled:          true,
		FailureThreshold: 3,
	}
	am := NewAlertManager(config)

	// First failure - should not trigger alert
	alert := am.ProcessFailure("test.com", "connection timeout")
	if alert != nil {
		t.Error("Expected no alert for first failure")
	}

	// Second failure - should not trigger alert
	alert = am.ProcessFailure("test.com", "connection timeout")
	if alert != nil {
		t.Error("Expected no alert for second failure")
	}

	// Check failure count
	if count := am.GetFailureCount("test.com"); count != 2 {
		t.Errorf("Expected failure count to be 2, got %d", count)
	}
}

func TestProcessFailure_ReachesThreshold(t *testing.T) {
	config := AlertConfig{
		Enabled:          true,
		FailureThreshold: 3,
	}
	am := NewAlertManager(config)

	// Reach threshold
	am.ProcessFailure("test.com", "error 1")
	am.ProcessFailure("test.com", "error 2")
	alert := am.ProcessFailure("test.com", "error 3")

	if alert == nil {
		t.Fatal("Expected alert when reaching threshold")
	}

	if alert.Type != AlertTypeFailure {
		t.Errorf("Expected alert type to be %s, got %s", AlertTypeFailure, alert.Type)
	}
	if alert.Severity != SeverityCritical {
		t.Errorf("Expected severity to be %s, got %s", SeverityCritical, alert.Severity)
	}
	if alert.Target != "test.com" {
		t.Errorf("Expected target to be test.com, got %s", alert.Target)
	}
	if alert.FailureCount != 3 {
		t.Errorf("Expected failure count to be 3, got %d", alert.FailureCount)
	}
	if alert.Details != "error 3" {
		t.Errorf("Expected details to be 'error 3', got %s", alert.Details)
	}

	// Check alert state
	if !am.IsInAlertState("test.com") {
		t.Error("Expected target to be in alert state")
	}
}

func TestProcessFailure_PreventsDuplicateAlerts(t *testing.T) {
	config := AlertConfig{
		Enabled:          true,
		FailureThreshold: 2,
	}
	am := NewAlertManager(config)

	// Reach threshold
	am.ProcessFailure("test.com", "error 1")
	alert1 := am.ProcessFailure("test.com", "error 2")

	if alert1 == nil {
		t.Fatal("Expected first alert when reaching threshold")
	}

	// Additional failure should not trigger another alert
	alert2 := am.ProcessFailure("test.com", "error 3")
	if alert2 != nil {
		t.Error("Expected no duplicate alert")
	}
}

func TestProcessSuccess_WithoutPreviousAlert(t *testing.T) {
	config := AlertConfig{
		Enabled: true,
	}
	am := NewAlertManager(config)

	// Success without previous failures should not trigger recovery alert
	alert := am.ProcessSuccess("test.com")
	if alert != nil {
		t.Error("Expected no recovery alert without previous alert state")
	}
}

func TestProcessSuccess_WithRecovery(t *testing.T) {
	config := AlertConfig{
		Enabled:          true,
		FailureThreshold: 2,
	}
	am := NewAlertManager(config)

	// Trigger alert state
	am.ProcessFailure("test.com", "error 1")
	am.ProcessFailure("test.com", "error 2")

	// Process success - should trigger recovery alert
	alert := am.ProcessSuccess("test.com")

	if alert == nil {
		t.Fatal("Expected recovery alert")
	}

	if alert.Type != AlertTypeRecovery {
		t.Errorf("Expected alert type to be %s, got %s", AlertTypeRecovery, alert.Type)
	}
	if alert.Severity != SeverityWarning {
		t.Errorf("Expected severity to be %s, got %s", SeverityWarning, alert.Severity)
	}
	if alert.Target != "test.com" {
		t.Errorf("Expected target to be test.com, got %s", alert.Target)
	}

	// Check that alert state is cleared
	if am.IsInAlertState("test.com") {
		t.Error("Expected target to not be in alert state after recovery")
	}
	if count := am.GetFailureCount("test.com"); count != 0 {
		t.Errorf("Expected failure count to be reset to 0, got %d", count)
	}
}

func TestProcessSSLAlert_Critical(t *testing.T) {
	config := AlertConfig{
		Enabled:         true,
		SSLWarningDays:  30,
		SSLCriticalDays: 7,
	}
	am := NewAlertManager(config)

	// SSL certificate expiring in 5 days (critical)
	alert := am.ProcessSSLAlert("secure.com", 5, "CN=secure.com, Issuer=Let's Encrypt")

	if alert == nil {
		t.Fatal("Expected SSL critical alert")
	}

	if alert.Type != AlertTypeSSL {
		t.Errorf("Expected alert type to be %s, got %s", AlertTypeSSL, alert.Type)
	}
	if alert.Severity != SeverityCritical {
		t.Errorf("Expected severity to be %s, got %s", SeverityCritical, alert.Severity)
	}
	if alert.Target != "secure.com" {
		t.Errorf("Expected target to be secure.com, got %s", alert.Target)
	}
}

func TestProcessSSLAlert_Warning(t *testing.T) {
	config := AlertConfig{
		Enabled:         true,
		SSLWarningDays:  30,
		SSLCriticalDays: 7,
	}
	am := NewAlertManager(config)

	// SSL certificate expiring in 20 days (warning)
	alert := am.ProcessSSLAlert("secure.com", 20, "CN=secure.com, Issuer=Let's Encrypt")

	if alert == nil {
		t.Fatal("Expected SSL warning alert")
	}

	if alert.Type != AlertTypeSSL {
		t.Errorf("Expected alert type to be %s, got %s", AlertTypeSSL, alert.Type)
	}
	if alert.Severity != SeverityWarning {
		t.Errorf("Expected severity to be %s, got %s", SeverityWarning, alert.Severity)
	}
}

func TestProcessSSLAlert_NoAlert(t *testing.T) {
	config := AlertConfig{
		Enabled:         true,
		SSLWarningDays:  30,
		SSLCriticalDays: 7,
	}
	am := NewAlertManager(config)

	// SSL certificate expiring in 60 days (no alert needed)
	alert := am.ProcessSSLAlert("secure.com", 60, "CN=secure.com, Issuer=Let's Encrypt")

	if alert != nil {
		t.Error("Expected no SSL alert for certificate expiring in 60 days")
	}
}

func TestProcessSSLAlert_PreventsDuplicates(t *testing.T) {
	config := AlertConfig{
		Enabled:         true,
		SSLWarningDays:  30,
		SSLCriticalDays: 7,
	}
	am := NewAlertManager(config)

	// First critical alert
	alert1 := am.ProcessSSLAlert("secure.com", 5, "CN=secure.com")
	if alert1 == nil {
		t.Fatal("Expected first SSL critical alert")
	}

	// Second critical alert within 24 hours should be suppressed
	alert2 := am.ProcessSSLAlert("secure.com", 5, "CN=secure.com")
	if alert2 != nil {
		t.Error("Expected duplicate SSL alert to be suppressed")
	}
}

func TestAlertManager_DisabledConfig(t *testing.T) {
	config := AlertConfig{
		Enabled:          false,
		FailureThreshold: 1,
	}
	am := NewAlertManager(config)

	// Should not trigger alerts when disabled
	alert := am.ProcessFailure("test.com", "error")
	if alert != nil {
		t.Error("Expected no alert when alerting is disabled")
	}

	alert = am.ProcessSuccess("test.com")
	if alert != nil {
		t.Error("Expected no recovery alert when alerting is disabled")
	}

	alert = am.ProcessSSLAlert("secure.com", 5, "details")
	if alert != nil {
		t.Error("Expected no SSL alert when alerting is disabled")
	}
}

func TestGetAlertStats(t *testing.T) {
	config := AlertConfig{
		Enabled:          true,
		FailureThreshold: 2,
	}
	am := NewAlertManager(config)

	// Trigger alert for one target
	am.ProcessFailure("test1.com", "error")
	am.ProcessFailure("test1.com", "error")

	stats := am.GetAlertStats()

	if stats["enabled"] != true {
		t.Errorf("Expected enabled to be true, got %v", stats["enabled"])
	}
	if stats["failure_threshold"] != 2 {
		t.Errorf("Expected failure_threshold to be 2, got %v", stats["failure_threshold"])
	}

	failingTargets, ok := stats["failing_targets"].([]string)
	if !ok {
		t.Fatal("Expected failing_targets to be []string")
	}
	if len(failingTargets) != 1 || failingTargets[0] != "test1.com" {
		t.Errorf("Expected failing_targets to contain test1.com, got %v", failingTargets)
	}
}

func TestReset(t *testing.T) {
	config := AlertConfig{
		Enabled:          true,
		FailureThreshold: 2,
	}
	am := NewAlertManager(config)

	// Set up some state
	am.ProcessFailure("test.com", "error")
	am.ProcessFailure("test.com", "error")
	am.ProcessSSLAlert("secure.com", 5, "details")

	// Reset should clear all state
	am.Reset()

	if count := am.GetFailureCount("test.com"); count != 0 {
		t.Errorf("Expected failure count to be 0 after reset, got %d", count)
	}
	if am.IsInAlertState("test.com") {
		t.Error("Expected target to not be in alert state after reset")
	}

	stats := am.GetAlertStats()
	failingTargets := stats["failing_targets"].([]string)
	if len(failingTargets) != 0 {
		t.Errorf("Expected no failing targets after reset, got %v", failingTargets)
	}
}

func TestNewWebhookNotifier(t *testing.T) {
	notifier := NewWebhookNotifier("https://hooks.example.com/webhook")

	if notifier.webhookURL != "https://hooks.example.com/webhook" {
		t.Errorf("Expected webhook URL to be set correctly")
	}
	if notifier.client == nil {
		t.Error("Expected HTTP client to be initialized")
	}
	if notifier.maxRetries != 3 {
		t.Errorf("Expected maxRetries to be 3, got %d", notifier.maxRetries)
	}
	if notifier.baseDelay != 1*time.Second {
		t.Errorf("Expected baseDelay to be 1s, got %v", notifier.baseDelay)
	}
}

func TestWebhookNotifier_SendAlert_Success(t *testing.T) {
	// Create a test server that returns success
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request method and headers
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}
		if r.Header.Get("User-Agent") != "netirk-webhook-notifier" {
			t.Errorf("Expected User-Agent netirk-webhook-notifier, got %s", r.Header.Get("User-Agent"))
		}

		// Verify payload structure
		var payload WebhookPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("Failed to decode webhook payload: %v", err)
		}

		if payload.Source != "netirk" {
			t.Errorf("Expected source to be 'netirk', got %s", payload.Source)
		}
		if payload.Alert.Target != "test.com" {
			t.Errorf("Expected alert target to be 'test.com', got %s", payload.Alert.Target)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	notifier := NewWebhookNotifier(server.URL)
	alert := &Alert{
		Type:      AlertTypeFailure,
		Severity:  SeverityCritical,
		Target:    "test.com",
		Message:   "Test alert",
		Timestamp: time.Now(),
	}

	err := notifier.SendAlert(alert)
	if err != nil {
		t.Errorf("Expected successful webhook delivery, got error: %v", err)
	}
}

func TestWebhookNotifier_SendAlert_EmptyURL(t *testing.T) {
	notifier := NewWebhookNotifier("")
	alert := &Alert{
		Type:    AlertTypeFailure,
		Target:  "test.com",
		Message: "Test alert",
	}

	err := notifier.SendAlert(alert)
	if err == nil {
		t.Error("Expected error for empty webhook URL")
	}
	if !strings.Contains(err.Error(), "webhook URL not configured") {
		t.Errorf("Expected 'webhook URL not configured' error, got: %v", err)
	}
}

func TestWebhookNotifier_SendAlert_ServerError_WithRetry(t *testing.T) {
	attemptCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attemptCount++
		if attemptCount <= 2 {
			// Return server error for first two attempts
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			// Return success on third attempt
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	notifier := NewWebhookNotifier(server.URL)
	// Set shorter delays for faster testing
	notifier.SetRetryConfig(3, 10*time.Millisecond)

	alert := &Alert{
		Type:    AlertTypeFailure,
		Target:  "test.com",
		Message: "Test alert",
	}

	err := notifier.SendAlert(alert)
	if err != nil {
		t.Errorf("Expected successful delivery after retries, got error: %v", err)
	}
	if attemptCount != 3 {
		t.Errorf("Expected 3 attempts, got %d", attemptCount)
	}
}

func TestWebhookNotifier_SendAlert_ClientError_NoRetry(t *testing.T) {
	attemptCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attemptCount++
		w.WriteHeader(http.StatusBadRequest) // 4xx error should not be retried
	}))
	defer server.Close()

	notifier := NewWebhookNotifier(server.URL)
	notifier.SetRetryConfig(3, 10*time.Millisecond)

	alert := &Alert{
		Type:    AlertTypeFailure,
		Target:  "test.com",
		Message: "Test alert",
	}

	err := notifier.SendAlert(alert)
	if err == nil {
		t.Error("Expected error for 4xx response")
	}
	if attemptCount != 1 {
		t.Errorf("Expected only 1 attempt for 4xx error, got %d", attemptCount)
	}
	if !strings.Contains(err.Error(), "non-success status: 400") {
		t.Errorf("Expected status code in error message, got: %v", err)
	}
}

func TestWebhookNotifier_SendAlert_MaxRetriesExceeded(t *testing.T) {
	attemptCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attemptCount++
		w.WriteHeader(http.StatusInternalServerError) // Always return server error
	}))
	defer server.Close()

	notifier := NewWebhookNotifier(server.URL)
	notifier.SetRetryConfig(2, 10*time.Millisecond) // Only 2 retries

	alert := &Alert{
		Type:    AlertTypeFailure,
		Target:  "test.com",
		Message: "Test alert",
	}

	err := notifier.SendAlert(alert)
	if err == nil {
		t.Error("Expected error after max retries exceeded")
	}
	if attemptCount != 3 { // Initial attempt + 2 retries
		t.Errorf("Expected 3 total attempts, got %d", attemptCount)
	}
	if !strings.Contains(err.Error(), "failed after 3 attempts") {
		t.Errorf("Expected retry count in error message, got: %v", err)
	}
}

func TestWebhookNotifier_SetRetryConfig(t *testing.T) {
	notifier := NewWebhookNotifier("https://example.com")
	notifier.SetRetryConfig(5, 2*time.Second)

	if notifier.maxRetries != 5 {
		t.Errorf("Expected maxRetries to be 5, got %d", notifier.maxRetries)
	}
	if notifier.baseDelay != 2*time.Second {
		t.Errorf("Expected baseDelay to be 2s, got %v", notifier.baseDelay)
	}
}

func TestWebhookNotifier_SetTimeout(t *testing.T) {
	notifier := NewWebhookNotifier("https://example.com")
	notifier.SetTimeout(10 * time.Second)

	if notifier.client.Timeout != 10*time.Second {
		t.Errorf("Expected client timeout to be 10s, got %v", notifier.client.Timeout)
	}
}

func TestWebhookPayload_JSONMarshaling(t *testing.T) {
	alert := &Alert{
		Type:         AlertTypeFailure,
		Severity:     SeverityCritical,
		Target:       "test.com",
		Message:      "Test alert message",
		Timestamp:    time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
		FailureCount: 3,
		Details:      "Connection timeout",
	}

	payload := WebhookPayload{
		Alert:  *alert,
		Source: "netirk",
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Failed to marshal webhook payload: %v", err)
	}

	// Verify JSON structure
	var unmarshaled WebhookPayload
	if err := json.Unmarshal(jsonData, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal webhook payload: %v", err)
	}

	if unmarshaled.Source != "netirk" {
		t.Errorf("Expected source 'netirk', got %s", unmarshaled.Source)
	}
	if unmarshaled.Alert.Type != AlertTypeFailure {
		t.Errorf("Expected alert type %s, got %s", AlertTypeFailure, unmarshaled.Alert.Type)
	}
	if unmarshaled.Alert.Target != "test.com" {
		t.Errorf("Expected target 'test.com', got %s", unmarshaled.Alert.Target)
	}
}

func TestAlertManager_NotifyAlert(t *testing.T) {
	receivedPayload := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPayload = true
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := AlertConfig{
		Enabled:          true,
		FailureThreshold: 1,
		WebhookURL:       server.URL,
	}
	am := NewAlertManager(config)

	alert := &Alert{
		Type:    AlertTypeFailure,
		Target:  "test.com",
		Message: "Test alert",
	}

	// Test with valid alert and webhook URL
	am.NotifyAlert(alert)

	// Give async operation time to complete
	time.Sleep(100 * time.Millisecond)

	if !receivedPayload {
		t.Error("Expected webhook payload to be received")
	}
}

func TestAlertManager_NotifyAlert_NoWebhookURL(t *testing.T) {
	config := AlertConfig{
		Enabled:          true,
		FailureThreshold: 1,
		WebhookURL:       "", // No webhook URL configured
	}
	am := NewAlertManager(config)

	alert := &Alert{
		Type:    AlertTypeFailure,
		Target:  "test.com",
		Message: "Test alert",
	}

	// Should not panic or error when no webhook URL is configured
	am.NotifyAlert(alert)
	am.NotifyAlert(nil) // Should handle nil alert gracefully
}

// Email Notifier Tests

func TestNewEmailNotifier(t *testing.T) {
	config := EmailConfig{
		SMTPHost:    "smtp.example.com",
		SMTPPort:    587,
		Username:    "user@example.com",
		Password:    "password",
		FromAddress: "alerts@example.com",
		FromName:    "Netirk Alerts",
		UseStartTLS: true,
	}

	notifier := NewEmailNotifier(config)

	if notifier.config.SMTPHost != "smtp.example.com" {
		t.Errorf("Expected SMTP host to be smtp.example.com, got %s", notifier.config.SMTPHost)
	}
	if notifier.config.SMTPPort != 587 {
		t.Errorf("Expected SMTP port to be 587, got %d", notifier.config.SMTPPort)
	}
	if notifier.maxRetries != 3 {
		t.Errorf("Expected maxRetries to be 3, got %d", notifier.maxRetries)
	}
	if notifier.baseDelay != 1*time.Second {
		t.Errorf("Expected baseDelay to be 1s, got %v", notifier.baseDelay)
	}
}

func TestEmailNotifier_ValidateConfig_Valid(t *testing.T) {
	config := EmailConfig{
		SMTPHost:    "smtp.example.com",
		SMTPPort:    587,
		FromAddress: "alerts@example.com",
	}

	notifier := NewEmailNotifier(config)
	err := notifier.ValidateConfig()

	if err != nil {
		t.Errorf("Expected valid config to pass validation, got error: %v", err)
	}
}

func TestEmailNotifier_ValidateConfig_MissingHost(t *testing.T) {
	config := EmailConfig{
		SMTPPort:    587,
		FromAddress: "alerts@example.com",
	}

	notifier := NewEmailNotifier(config)
	err := notifier.ValidateConfig()

	if err == nil {
		t.Error("Expected validation error for missing SMTP host")
	}
	if !strings.Contains(err.Error(), "SMTP host is required") {
		t.Errorf("Expected 'SMTP host is required' error, got: %v", err)
	}
}

func TestEmailNotifier_ValidateConfig_InvalidPort(t *testing.T) {
	config := EmailConfig{
		SMTPHost:    "smtp.example.com",
		SMTPPort:    0,
		FromAddress: "alerts@example.com",
	}

	notifier := NewEmailNotifier(config)
	err := notifier.ValidateConfig()

	if err == nil {
		t.Error("Expected validation error for invalid port")
	}
	if !strings.Contains(err.Error(), "SMTP port must be between 1 and 65535") {
		t.Errorf("Expected port validation error, got: %v", err)
	}
}

func TestEmailNotifier_ValidateConfig_MissingFromAddress(t *testing.T) {
	config := EmailConfig{
		SMTPHost: "smtp.example.com",
		SMTPPort: 587,
	}

	notifier := NewEmailNotifier(config)
	err := notifier.ValidateConfig()

	if err == nil {
		t.Error("Expected validation error for missing from address")
	}
	if !strings.Contains(err.Error(), "from address is required") {
		t.Errorf("Expected 'from address is required' error, got: %v", err)
	}
}

func TestEmailNotifier_ValidateConfig_InvalidFromAddress(t *testing.T) {
	config := EmailConfig{
		SMTPHost:    "smtp.example.com",
		SMTPPort:    587,
		FromAddress: "invalid-email",
	}

	notifier := NewEmailNotifier(config)
	err := notifier.ValidateConfig()

	if err == nil {
		t.Error("Expected validation error for invalid from address")
	}
	if !strings.Contains(err.Error(), "from address must be a valid email address") {
		t.Errorf("Expected email validation error, got: %v", err)
	}
}

func TestEmailNotifier_FormatAlertEmail_Failure(t *testing.T) {
	config := EmailConfig{
		FromName: "Netirk Alerts",
	}
	notifier := NewEmailNotifier(config)

	alert := &Alert{
		Type:         AlertTypeFailure,
		Severity:     SeverityCritical,
		Target:       "api.example.com",
		Message:      "Target has failed 3 consecutive times",
		Timestamp:    time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		FailureCount: 3,
		Details:      "Connection timeout after 30s",
	}

	recipients := []string{"admin@example.com"}
	subject, body, err := notifier.formatAlertEmail(alert, recipients)

	if err != nil {
		t.Fatalf("Expected successful email formatting, got error: %v", err)
	}

	expectedSubject := "[CRITICAL] Netirk Alert - api.example.com"
	if subject != expectedSubject {
		t.Errorf("Expected subject '%s', got '%s'", expectedSubject, subject)
	}

	// Check that body contains key information
	if !strings.Contains(body, "Alert Type: FAILURE") {
		t.Error("Expected body to contain alert type")
	}
	if !strings.Contains(body, "Severity: CRITICAL") {
		t.Error("Expected body to contain severity")
	}
	if !strings.Contains(body, "Target: api.example.com") {
		t.Error("Expected body to contain target")
	}
	if !strings.Contains(body, "Failure Count: 3") {
		t.Error("Expected body to contain failure count")
	}
	if !strings.Contains(body, "Connection timeout after 30s") {
		t.Error("Expected body to contain details")
	}
	if !strings.Contains(body, "2024-01-15 10:30:00 UTC") {
		t.Error("Expected body to contain formatted timestamp")
	}
}

func TestEmailNotifier_FormatAlertEmail_Recovery(t *testing.T) {
	config := EmailConfig{}
	notifier := NewEmailNotifier(config)

	alert := &Alert{
		Type:      AlertTypeRecovery,
		Severity:  SeverityWarning,
		Target:    "api.example.com",
		Message:   "Target api.example.com has recovered",
		Timestamp: time.Date(2024, 1, 15, 11, 0, 0, 0, time.UTC),
	}

	recipients := []string{"admin@example.com"}
	subject, body, err := notifier.formatAlertEmail(alert, recipients)

	if err != nil {
		t.Fatalf("Expected successful email formatting, got error: %v", err)
	}

	expectedSubject := "[RECOVERY] Netirk Alert - api.example.com"
	if subject != expectedSubject {
		t.Errorf("Expected subject '%s', got '%s'", expectedSubject, subject)
	}

	if !strings.Contains(body, "Alert Type: RECOVERY") {
		t.Error("Expected body to contain recovery alert type")
	}
	if !strings.Contains(body, "The target has recovered and is now responding normally") {
		t.Error("Expected body to contain recovery message")
	}
}

func TestEmailNotifier_FormatAlertEmail_SSL(t *testing.T) {
	config := EmailConfig{}
	notifier := NewEmailNotifier(config)

	alert := &Alert{
		Type:      AlertTypeSSL,
		Severity:  SeverityCritical,
		Target:    "secure.example.com",
		Message:   "SSL certificate for secure.example.com expires in 5 days",
		Timestamp: time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC),
		Details:   "CN=secure.example.com, Issuer=Let's Encrypt",
	}

	recipients := []string{"security@example.com"}
	subject, body, err := notifier.formatAlertEmail(alert, recipients)

	if err != nil {
		t.Fatalf("Expected successful email formatting, got error: %v", err)
	}

	expectedSubject := "[SSL CRITICAL] Netirk Alert - secure.example.com"
	if subject != expectedSubject {
		t.Errorf("Expected subject '%s', got '%s'", expectedSubject, subject)
	}

	if !strings.Contains(body, "Alert Type: SSL") {
		t.Error("Expected body to contain SSL alert type")
	}
	if !strings.Contains(body, "SSL Certificate Action Required") {
		t.Error("Expected body to contain SSL action guidance")
	}
	if !strings.Contains(body, "Certificate expires very soon - immediate action required") {
		t.Error("Expected body to contain critical SSL guidance")
	}
	if !strings.Contains(body, "CN=secure.example.com, Issuer=Let's Encrypt") {
		t.Error("Expected body to contain certificate details")
	}
}

func TestEmailNotifier_FormatAlertEmail_SSLWarning(t *testing.T) {
	config := EmailConfig{}
	notifier := NewEmailNotifier(config)

	alert := &Alert{
		Type:      AlertTypeSSL,
		Severity:  SeverityWarning,
		Target:    "secure.example.com",
		Message:   "SSL certificate for secure.example.com expires in 20 days",
		Timestamp: time.Now(),
	}

	recipients := []string{"security@example.com"}
	subject, body, err := notifier.formatAlertEmail(alert, recipients)

	if err != nil {
		t.Fatalf("Expected successful email formatting, got error: %v", err)
	}

	expectedSubject := "[SSL WARNING] Netirk Alert - secure.example.com"
	if subject != expectedSubject {
		t.Errorf("Expected subject '%s', got '%s'", expectedSubject, subject)
	}

	if !strings.Contains(body, "Certificate expires soon - plan renewal") {
		t.Error("Expected body to contain warning SSL guidance")
	}
}

func TestEmailNotifier_FormatAlertEmail_NilAlert(t *testing.T) {
	config := EmailConfig{}
	notifier := NewEmailNotifier(config)

	recipients := []string{"admin@example.com"}
	_, _, err := notifier.formatAlertEmail(nil, recipients)

	if err == nil {
		t.Error("Expected error for nil alert")
	}
	if !strings.Contains(err.Error(), "alert cannot be nil") {
		t.Errorf("Expected 'alert cannot be nil' error, got: %v", err)
	}
}

func TestEmailNotifier_SendAlert_InvalidConfig(t *testing.T) {
	config := EmailConfig{
		SMTPHost: "", // Invalid config
	}
	notifier := NewEmailNotifier(config)

	alert := &Alert{
		Type:    AlertTypeFailure,
		Target:  "test.com",
		Message: "Test alert",
	}

	recipients := []string{"admin@example.com"}
	err := notifier.SendAlert(alert, recipients)

	if err == nil {
		t.Error("Expected error for invalid config")
	}
	if !strings.Contains(err.Error(), "email configuration invalid") {
		t.Errorf("Expected configuration error, got: %v", err)
	}
}

func TestEmailNotifier_SendAlert_NoRecipients(t *testing.T) {
	config := EmailConfig{
		SMTPHost:    "smtp.example.com",
		SMTPPort:    587,
		FromAddress: "alerts@example.com",
	}
	notifier := NewEmailNotifier(config)

	alert := &Alert{
		Type:    AlertTypeFailure,
		Target:  "test.com",
		Message: "Test alert",
	}

	err := notifier.SendAlert(alert, []string{})

	if err == nil {
		t.Error("Expected error for no recipients")
	}
	if !strings.Contains(err.Error(), "no email recipients specified") {
		t.Errorf("Expected 'no email recipients specified' error, got: %v", err)
	}
}

func TestEmailNotifier_SendAlert_InvalidRecipient(t *testing.T) {
	config := EmailConfig{
		SMTPHost:    "smtp.example.com",
		SMTPPort:    587,
		FromAddress: "alerts@example.com",
	}
	notifier := NewEmailNotifier(config)

	alert := &Alert{
		Type:    AlertTypeFailure,
		Target:  "test.com",
		Message: "Test alert",
	}

	recipients := []string{"invalid-email"}
	err := notifier.SendAlert(alert, recipients)

	if err == nil {
		t.Error("Expected error for invalid recipient")
	}
	if !strings.Contains(err.Error(), "invalid email address") {
		t.Errorf("Expected invalid email address error, got: %v", err)
	}
}

func TestEmailNotifier_SetRetryConfig(t *testing.T) {
	config := EmailConfig{
		SMTPHost:    "smtp.example.com",
		SMTPPort:    587,
		FromAddress: "alerts@example.com",
	}
	notifier := NewEmailNotifier(config)

	notifier.SetRetryConfig(5, 2*time.Second)

	if notifier.maxRetries != 5 {
		t.Errorf("Expected maxRetries to be 5, got %d", notifier.maxRetries)
	}
	if notifier.baseDelay != 2*time.Second {
		t.Errorf("Expected baseDelay to be 2s, got %v", notifier.baseDelay)
	}
}

func TestAlertManager_NotifyAlert_WithEmail(t *testing.T) {
	config := AlertConfig{
		Enabled:          true,
		FailureThreshold: 1,
		EmailRecipients:  []string{"admin@example.com"},
		EmailConfig: EmailConfig{
			SMTPHost:    "smtp.example.com",
			SMTPPort:    587,
			FromAddress: "alerts@example.com",
		},
	}
	am := NewAlertManager(config)

	alert := &Alert{
		Type:    AlertTypeFailure,
		Target:  "test.com",
		Message: "Test alert",
	}

	// Should not panic when email is configured (actual sending will fail in test environment)
	am.NotifyAlert(alert)

	// Give async operation time to complete
	time.Sleep(100 * time.Millisecond)
}

func TestAlertManager_NotifyAlert_BothWebhookAndEmail(t *testing.T) {
	// Create test webhook server
	webhookReceived := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		webhookReceived = true
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := AlertConfig{
		Enabled:          true,
		FailureThreshold: 1,
		WebhookURL:       server.URL,
		EmailRecipients:  []string{"admin@example.com"},
		EmailConfig: EmailConfig{
			SMTPHost:    "smtp.example.com",
			SMTPPort:    587,
			FromAddress: "alerts@example.com",
		},
	}
	am := NewAlertManager(config)

	alert := &Alert{
		Type:    AlertTypeFailure,
		Target:  "test.com",
		Message: "Test alert",
	}

	// Should attempt both webhook and email notifications
	am.NotifyAlert(alert)

	// Give async operations time to complete
	time.Sleep(200 * time.Millisecond)

	if !webhookReceived {
		t.Error("Expected webhook to be received")
	}
}