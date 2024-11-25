package cmd

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var TargetUrl string

func CheckTCPConnection(TargetUrl string) {
	log.Print("Testing the url: ", TargetUrl)
	var dialer net.Dialer
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)

	defer cancel()

	conn, err := dialer.DialContext(ctx, "tcp", TargetUrl)
	if err != nil {
		log.Fatalf("Failed to dial: %v", err)
	} else {
		log.Print("TCP success!")
	}
	defer conn.Close()
}

func CheckHttpConnection(TargetUrl string) {
	log.Print("Testing the url: ", TargetUrl)
	r, e := http.Get(TargetUrl)

	if e != nil {
		fmt.Print(e)
		os.Exit(1)
	}
	statusCode := r.StatusCode

	if statusCode == 200 {
		log.Print("Success, the endpoint is reachable!")
	} else if statusCode == 403 {
		log.Print("\n", "Server reachable but access denied\n")
	} else {
		log.Print(statusCode, "\n", "Raw response: \n\n", r)
	}
}

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

// addCmd represents the add command
var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Verify if host is reachable",
	Long:  `verify if host resp is OK`,
	Run: func(cmd *cobra.Command, args []string) {
		host, _ := cmd.Flags().GetString("target")
		hostIp, _ := cmd.Flags().GetString("ip")
		port, _ := cmd.Flags().GetInt("port")
		sslValidate, _ := cmd.Flags().GetBool("verify-ssl")
		if sslValidate {
			fmt.Println("Getting server certs...")
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
		} else {
			if hostIp == "" && !strings.Contains(host, "http") {
				TargetUrl = hostIp + ":" + strconv.Itoa(port)
				CheckTCPConnection(TargetUrl)
			} else if hostIp == "" || strings.Contains(hostIp, "https") || strings.Contains(host, "http") {
				TargetUrl = host + ":" + strconv.Itoa(port)
				CheckHttpConnection(TargetUrl)
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(checkCmd)
	checkCmd.Flags().String("target", "google.com", "host name like: google.com")
	checkCmd.Flags().String("ip", "0.0.0.0", "IP address of the host like: 127.0.0.1")
	checkCmd.Flags().Int("port", 443, "port number to test: 443")
	checkCmd.Flags().Bool("verify-ssl", false, "Print server certs")
}
