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
	"time"

	"github.com/spf13/cobra"
)

const appName string = "| Netirk |"

var TargetUrl string
var bannerLines = strings.Repeat("-", len(appName))

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

// addCmd represents the add command
var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Verify if host is reachable",
	Long:  `verify if host resp is OK`,
	Run: func(cmd *cobra.Command, args []string) {
		host, _ := cmd.Flags().GetString("hostname")
		hostIp, _ := cmd.Flags().GetString("host-ip")
		port, _ := cmd.Flags().GetInt("hostport")
		log.Print(bannerLines)
		log.Print(appName)
		log.Print(bannerLines)

		if host == "" && !strings.Contains(hostIp, "https") && !strings.Contains(host, "http") {
			TargetUrl = hostIp + ":" + strconv.Itoa(port)
			CheckTCPConnection(TargetUrl)
		} else if hostIp == "" || strings.Contains(hostIp, "https") || strings.Contains(host, "http") {
			TargetUrl = host + ":" + strconv.Itoa(port)
			CheckHttpConnection(TargetUrl)
		}
	},
}

func init() {
	rootCmd.AddCommand(checkCmd)
	checkCmd.Flags().String("hostname", "", "host name like: google.com")
	checkCmd.Flags().String("host-ip", "", "IP address of the host like: 127.0.0.1")
	checkCmd.Flags().Int("hostport", 443, "port number to test: 443")
}
