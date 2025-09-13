package helpers

import (
	"testing"
	"time"
)

func TestGetSSLWarningLevel(t *testing.T) {
	tests := []struct {
		name         string
		daysToExpiry int
		expected     string
	}{
		{"Expired certificate", -5, "expired"},
		{"Certificate expires today", 0, "expired"},
		{"Critical - 3 days", 3, "critical"},
		{"Critical - 7 days", 7, "critical"},
		{"Warning - 15 days", 15, "warning"},
		{"Warning - 30 days", 30, "warning"},
		{"OK - 60 days", 60, "ok"},
		{"OK - 365 days", 365, "ok"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetSSLWarningLevel(tt.daysToExpiry)
			if result != tt.expected {
				t.Errorf("GetSSLWarningLevel(%d) = %s, want %s", tt.daysToExpiry, result, tt.expected)
			}
		})
	}
}

func TestFormatSSLWarning(t *testing.T) {
	target := "https://example.com"

	tests := []struct {
		name     string
		sslData  *SSLData
		expected string
	}{
		{
			name:     "Nil SSL data",
			sslData:  nil,
			expected: "",
		},
		{
			name: "Invalid certificate with error",
			sslData: &SSLData{
				IsValid: false,
				Error:   "certificate has expired",
			},
			expected: "SSL certificate error for https://example.com: certificate has expired",
		},
		{
			name: "Invalid certificate without error",
			sslData: &SSLData{
				IsValid: false,
			},
			expected: "SSL certificate invalid for https://example.com",
		},
		{
			name: "Expired certificate",
			sslData: &SSLData{
				IsValid:      true,
				DaysToExpiry: -5,
			},
			expected: "SSL certificate EXPIRED for https://example.com (expired 5 days ago)",
		},
		{
			name: "Critical certificate",
			sslData: &SSLData{
				IsValid:      true,
				DaysToExpiry: 3,
			},
			expected: "SSL certificate CRITICAL for https://example.com (expires in 3 days)",
		},
		{
			name: "Warning certificate",
			sslData: &SSLData{
				IsValid:      true,
				DaysToExpiry: 15,
			},
			expected: "SSL certificate WARNING for https://example.com (expires in 15 days)",
		},
		{
			name: "OK certificate",
			sslData: &SSLData{
				IsValid:      true,
				DaysToExpiry: 60,
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatSSLWarning(tt.sslData, target)
			if result != tt.expected {
				t.Errorf("FormatSSLWarning() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestShouldTriggerSSLAlert(t *testing.T) {
	tests := []struct {
		name         string
		sslData      *SSLData
		warningDays  int
		criticalDays int
		expected     bool
	}{
		{
			name:         "Nil SSL data",
			sslData:      nil,
			warningDays:  30,
			criticalDays: 7,
			expected:     true,
		},
		{
			name: "Invalid certificate",
			sslData: &SSLData{
				IsValid: false,
			},
			warningDays:  30,
			criticalDays: 7,
			expected:     true,
		},
		{
			name: "Certificate expires within warning period",
			sslData: &SSLData{
				IsValid:      true,
				DaysToExpiry: 15,
			},
			warningDays:  30,
			criticalDays: 7,
			expected:     true,
		},
		{
			name: "Certificate expires within critical period",
			sslData: &SSLData{
				IsValid:      true,
				DaysToExpiry: 3,
			},
			warningDays:  30,
			criticalDays: 7,
			expected:     true,
		},
		{
			name: "Certificate is OK",
			sslData: &SSLData{
				IsValid:      true,
				DaysToExpiry: 60,
			},
			warningDays:  30,
			criticalDays: 7,
			expected:     false,
		},
		{
			name: "Certificate expires exactly at warning threshold",
			sslData: &SSLData{
				IsValid:      true,
				DaysToExpiry: 30,
			},
			warningDays:  30,
			criticalDays: 7,
			expected:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ShouldTriggerSSLAlert(tt.sslData, tt.warningDays, tt.criticalDays)
			if result != tt.expected {
				t.Errorf("ShouldTriggerSSLAlert() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestSSLCertificateInfo(t *testing.T) {
	// Test the SSLCertificateInfo structure
	now := time.Now()
	expiryDate := now.AddDate(0, 0, 30) // 30 days from now

	certInfo := SSLCertificateInfo{
		Position:     0,
		Subject:      "example.com",
		Issuer:       "Let's Encrypt Authority X3",
		SerialNumber: "123456789",
		NotBefore:    now.AddDate(0, 0, -90), // 90 days ago
		NotAfter:     expiryDate,
		DaysToExpiry: 30,
		DNSNames:     []string{"example.com", "www.example.com"},
		IsCA:         false,
		IsValid:      true,
	}

	// Verify the structure fields
	if certInfo.Subject != "example.com" {
		t.Errorf("Expected Subject to be 'example.com', got %s", certInfo.Subject)
	}

	if certInfo.DaysToExpiry != 30 {
		t.Errorf("Expected DaysToExpiry to be 30, got %d", certInfo.DaysToExpiry)
	}

	if !certInfo.IsValid {
		t.Errorf("Expected IsValid to be true")
	}

	if len(certInfo.DNSNames) != 2 {
		t.Errorf("Expected 2 DNS names, got %d", len(certInfo.DNSNames))
	}
}