package cmd

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/http/httptrace"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(traceCmd)
	traceCmd.Flags().String("host", "https://google.com", "Hostname to run tests")
	traceCmd.Flags().Int("port", 443, "Port number if testing non standard ports")
}

var traceCmd = &cobra.Command{
	Use:   "trace",
	Short: "Run network trace, provides metrics for DNS and initial client connection.",
	Run: func(cmd *cobra.Command, args []string) {
		serverPort, _ := cmd.Flags().GetInt("port")
		serverAddr, _ := cmd.Flags().GetString("host")
		if serverPort != 443 && strings.Contains(serverAddr, "https") {
			serverAddr = serverAddr + ":" + strconv.Itoa(serverPort)
		}
		req, _ := http.NewRequest("GET", serverAddr, nil)
		var start, connect, tlsHandshake, dns time.Time
		trace := &httptrace.ClientTrace{
			DNSStart: func(_ httptrace.DNSStartInfo) {
				dns = time.Now()
			},
			DNSDone: func(_ httptrace.DNSDoneInfo) {
				fmt.Printf("DNS Resolution done: %v\n", time.Since(dns))
			},
			ConnectStart: func(_, _ string) {
				connect = time.Now()
			},
			ConnectDone: func(_, _ string, _ error) {
				fmt.Printf("Connect Done: %v\n", time.Since(connect))
			},
			TLSHandshakeStart: func() {
				tlsHandshake = time.Now()
			},
			TLSHandshakeDone: func(_ tls.ConnectionState, _ error) {
				fmt.Printf("TLS Handshake Done: %v\n", time.Since(tlsHandshake))
			},
			GotFirstResponseByte: func() {
				fmt.Printf("Time to first byte: %v\n", time.Since(start))
			},
		}

		req = req.WithContext(
			httptrace.WithClientTrace(
				req.Context(),
				trace,
			))
		start = time.Now()
		if _, err := http.DefaultTransport.RoundTrip(req); err != nil {
			fmt.Printf("Request failed: %v\n", err)
		}
	},
}
