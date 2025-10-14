package helpers

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"log"
	"net/url"
	"strings"
	"time"
)

type serverCertificateData struct {
	IsCA           bool
	Issuer         string
	DNSNames       []string
	ExpirationTime string
	PublicKey      string
}

// SSLCertificateInfo contains detailed information about a single certificate in the chain
type SSLCertificateInfo struct {
	Position     int       `json:"position"`
	Subject      string    `json:"subject"`
	Issuer       string    `json:"issuer"`
	SerialNumber string    `json:"serial_number"`
	NotBefore    time.Time `json:"not_before"`
	NotAfter     time.Time `json:"not_after"`
	DaysToExpiry int       `json:"days_to_expiry"`
	DNSNames     []string  `json:"dns_names"`
	IsCA         bool      `json:"is_ca"`
	IsValid      bool      `json:"is_valid"`
	KeyUsage     x509.KeyUsage `json:"key_usage"`
}

func parseCertificateData(certificateData *x509.Certificate) serverCertificateData {

	pemFormat := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certificateData.Raw,
	})

	s := serverCertificateData{
		IsCA:           certificateData.IsCA,
		DNSNames:       certificateData.DNSNames,
		Issuer:         certificateData.Issuer.CommonName,
		ExpirationTime: certificateData.NotAfter.Format(time.RFC850),
		PublicKey:      string(pemFormat),
	}

	return s
}

// CheckSSLCertificate performs SSL certificate monitoring and returns SSL data
func CheckSSLCertificate(targetURL string) *SSLData {
	sslData := &SSLData{
		IsValid: false,
	}

	// Parse the URL to extract host and port
	parsedURL, err := url.Parse(targetURL)
	if err != nil {
		sslData.Error = fmt.Sprintf("failed to parse URL: %v", err)
		return sslData
	}

	host := parsedURL.Hostname()
	port := parsedURL.Port()
	if port == "" {
		port = "443" // Default HTTPS port
	}

	// Establish TLS connection
	config := &tls.Config{
		ServerName: host,
	}

	conn, err := tls.Dial("tcp", host+":"+port, config)
	if err != nil {
		sslData.Error = fmt.Sprintf("TLS connection failed: %v", err)
		return sslData
	}
	defer conn.Close()

	// Get the peer certificates
	certs := conn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		sslData.Error = "no certificates found"
		return sslData
	}

	// Analyze the first certificate (server certificate)
	cert := certs[0]
	
	// Calculate days to expiry
	now := time.Now()
	daysToExpiry := int(cert.NotAfter.Sub(now).Hours() / 24)

	sslData.ExpiryDate = cert.NotAfter
	sslData.DaysToExpiry = daysToExpiry
	sslData.Issuer = cert.Issuer.CommonName
	sslData.IsValid = now.Before(cert.NotAfter) && now.After(cert.NotBefore)

	// Check for certificate validation errors
	if err := cert.VerifyHostname(host); err != nil {
		sslData.Error = fmt.Sprintf("hostname verification failed: %v", err)
		sslData.IsValid = false
	}

	return sslData
}

// GetSSLWarningLevel returns the warning level based on days to expiry
func GetSSLWarningLevel(daysToExpiry int) string {
	if daysToExpiry <= 0 {
		return "expired"
	} else if daysToExpiry <= 7 {
		return "critical"
	} else if daysToExpiry <= 30 {
		return "warning"
	}
	return "ok"
}

// FormatSSLWarning returns a formatted warning message for SSL certificate status
func FormatSSLWarning(sslData *SSLData, target string) string {
	if sslData == nil {
		return ""
	}

	if !sslData.IsValid {
		if sslData.Error != "" {
			return fmt.Sprintf("SSL certificate error for %s: %s", target, sslData.Error)
		}
		return fmt.Sprintf("SSL certificate invalid for %s", target)
	}

	warningLevel := GetSSLWarningLevel(sslData.DaysToExpiry)
	switch warningLevel {
	case "expired":
		return fmt.Sprintf("SSL certificate EXPIRED for %s (expired %d days ago)", target, -sslData.DaysToExpiry)
	case "critical":
		return fmt.Sprintf("SSL certificate CRITICAL for %s (expires in %d days)", target, sslData.DaysToExpiry)
	case "warning":
		return fmt.Sprintf("SSL certificate WARNING for %s (expires in %d days)", target, sslData.DaysToExpiry)
	default:
		return ""
	}
}

// ShouldTriggerSSLAlert determines if an SSL alert should be triggered
func ShouldTriggerSSLAlert(sslData *SSLData, warningDays, criticalDays int) bool {
	if sslData == nil || !sslData.IsValid {
		return true
	}

	return sslData.DaysToExpiry <= warningDays
}

// GetSSLCertificateChain returns detailed information about the entire certificate chain
func GetSSLCertificateChain(targetURL string) ([]SSLCertificateInfo, error) {
	// Parse the URL to extract host and port
	parsedURL, err := url.Parse(targetURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL: %v", err)
	}

	host := parsedURL.Hostname()
	port := parsedURL.Port()
	if port == "" {
		port = "443" // Default HTTPS port
	}

	// Establish TLS connection
	config := &tls.Config{
		ServerName: host,
	}

	conn, err := tls.Dial("tcp", host+":"+port, config)
	if err != nil {
		return nil, fmt.Errorf("TLS connection failed: %v", err)
	}
	defer conn.Close()

	// Get the peer certificates
	certs := conn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		return nil, fmt.Errorf("no certificates found")
	}

	var certChain []SSLCertificateInfo
	for i, cert := range certs {
		certInfo := SSLCertificateInfo{
			Position:     i,
			Subject:      cert.Subject.CommonName,
			Issuer:       cert.Issuer.CommonName,
			SerialNumber: cert.SerialNumber.String(),
			NotBefore:    cert.NotBefore,
			NotAfter:     cert.NotAfter,
			DNSNames:     cert.DNSNames,
			IsCA:         cert.IsCA,
			KeyUsage:     cert.KeyUsage,
		}

		// Calculate days to expiry
		now := time.Now()
		certInfo.DaysToExpiry = int(cert.NotAfter.Sub(now).Hours() / 24)
		certInfo.IsValid = now.Before(cert.NotAfter) && now.After(cert.NotBefore)

		certChain = append(certChain, certInfo)
	}

	return certChain, nil
}

func SslReportOutput(host string) {
	fmt.Println("Getting server certs...")
	host = strings.Trim(host, "https://")
	connect, err := tls.Dial("tcp", host+":443", nil)

	if err != nil {
		log.Panic("No SSL support for server:\n" + err.Error())
	}

	defer connect.Close()

	for i, cer := range connect.ConnectionState().PeerCertificates {
		certData := parseCertificateData(cer)
		fmt.Printf(`
➥ Cert: %d 
￫ CA: %t
￫ Issuer: %s
￫ Expiry: %s
￫ PublicKey: 
%s`, i, certData.IsCA, certData.Issuer, certData.ExpirationTime, certData.PublicKey)
	}
}
