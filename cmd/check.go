package cmd

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/himasagaratluri/netirk/cmd/helpers"
	"github.com/spf13/cobra"
)

var TargetUrl string
var wg sync.WaitGroup

func CheckTCPConnection(TargetUrl string) string {
	log.Print("Testing the url: ", TargetUrl)
	var dialer net.Dialer
	// var tcpStatus string
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)

	defer cancel()

	conn, err := dialer.DialContext(ctx, "tcp", TargetUrl)

	if err != nil {
		log.Fatalf("Failed to dial: %v", err)
		tcpStatus := "FAILED"
		return tcpStatus
	} else {
		defer conn.Close()
		log.Print("TCP success!")
		tcpStatus := "SUCCESS"
		return tcpStatus
	}
}

func CheckHttpConnection(TargetUrl string) int {
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

	return statusCode
}

func checkMain(host, hostIp string, port int, sslValidate bool) {
	defer wg.Done()
	if sslValidate {
		helpers.SslReportOutput(host)
	} else {
		if hostIp == "" && !strings.Contains(host, "http") {
			TargetUrl = hostIp + ":" + strconv.Itoa(port)
			CheckTCPConnection(TargetUrl)
		} else if hostIp == "" || strings.Contains(hostIp, "https") || strings.Contains(host, "http") {
			TargetUrl = host + ":" + strconv.Itoa(port)
			CheckHttpConnection(TargetUrl)
		}
	}
}

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Verify if host is reachable",
	Long:  `verify if host resp is OK`,
	Run: func(cmd *cobra.Command, args []string) {
		targetsFilePath, _ := cmd.Flags().GetString("targets-file")
		hosts := helpers.ParseTargetFile(targetsFilePath)
		host, _ := cmd.Flags().GetString("target")
		hostIp, _ := cmd.Flags().GetString("ip")
		port, _ := cmd.Flags().GetInt("port")
		sslValidate, _ := cmd.Flags().GetBool("verify-ssl")
		hosts.Targets = append(hosts.Targets, host)

		for _, host := range hosts.Targets {
			wg.Add(1)
			go checkMain(host, hostIp, port, sslValidate)

		}
		wg.Wait()
	},
}

func init() {
	rootCmd.AddCommand(checkCmd)
	checkCmd.Flags().String("target", "google.com", "host name like: google.com")
	checkCmd.Flags().String("targets-file", "", "Path to the yaml file that contains target hosts.")
	checkCmd.Flags().String("ip", "0.0.0.0", "IP address of the host like: 127.0.0.1")
	checkCmd.Flags().Int("port", 443, "port number to test: 443")
	checkCmd.Flags().Bool("verify-ssl", false, "Print server certs")
}
