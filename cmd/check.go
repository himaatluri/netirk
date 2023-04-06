package cmd

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

const appName string = "Netirk"

// var (
// 	host = "https://google.com"
// 	port = 443
// )

var bannerLines = strings.Repeat("-", len(appName))

// addCmd represents the add command
var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Verify if host is reachable",
	Long:  `verify if host resp is OK`,
	Run: func(cmd *cobra.Command, args []string) {
		host := targetHost.hostname
		port := targetHost.hostport
		log.Print(bannerLines)
		log.Print(appName)
		log.Print(bannerLines)
		url := host + ":" + strconv.Itoa(port)
		log.Print("Testing the url: ", url)
		r, e := http.Get(url)

		if e != nil {
			fmt.Print(e)
			os.Exit(1)
		}
		statusCode := r.StatusCode

		if statusCode == 200 {
			log.Print("Success, the endpoint is reachable!")
		} else {
			log.Print(statusCode, "\n", "Raw response: \n\n", r)
		}
	},
}

func init() {
	rootCmd.AddCommand(checkCmd)
	checkCmd.Flags().String("hostname", "https://google.com", "host name like: google.com")
	checkCmd.Flags().Int("hostport", 443, "port number to test: 443")
}
