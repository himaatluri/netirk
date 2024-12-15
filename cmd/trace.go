package cmd

import (
	"crypto/tls"
	"encoding/json"
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

type traceData struct {
	dnsInfo          string
	connectionStatus string
	tlsInfo          string
	firstByteInfo    string
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
		td := traceData{}
		trace := &httptrace.ClientTrace{
			DNSStart: func(_ httptrace.DNSStartInfo) {
				dns = time.Now()
			},
			DNSDone: func(_ httptrace.DNSDoneInfo) {
				fmt.Printf("DNS Resolution done: %v\n", time.Since(dns))
				td.dnsInfo = time.Since(dns).String()
			},
			ConnectStart: func(_, _ string) {
				connect = time.Now()
			},
			ConnectDone: func(_, _ string, _ error) {
				fmt.Printf("Connect Done: %v\n", time.Since(connect))
				td.connectionStatus = time.Since(connect).String()
			},
			TLSHandshakeStart: func() {
				tlsHandshake = time.Now()
			},
			TLSHandshakeDone: func(_ tls.ConnectionState, _ error) {
				fmt.Printf("TLS Handshake Done: %v\n", time.Since(tlsHandshake))
				td.tlsInfo = time.Since(tlsHandshake).String()
			},
			GotFirstResponseByte: func() {
				fmt.Printf("Time to first byte: %v\n", time.Since(start))
				td.firstByteInfo = time.Since(start).String()
			},
		}

		req = req.WithContext(
			httptrace.WithClientTrace(
				req.Context(),
				trace,
			),
		)

		start = time.Now()

		if _, err := http.DefaultTransport.RoundTrip(req); err != nil {
			fmt.Printf("Request failed: %v\n", err)
		}

		op, _ := rootCmd.Flags().GetString("output")

		if op == "json" {
			jo, err := json.Marshal(td)
			if err != nil {
				fmt.Println(err)
			}

			fmt.Println(jo)
		}
	},
}
