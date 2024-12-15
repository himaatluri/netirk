package helpers

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"log"
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
