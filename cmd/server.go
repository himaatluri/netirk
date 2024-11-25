package cmd

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(serverCmd)
	serverCmd.Flags().Int("port", 8080, "port number to listen: defaults to 8080")
}

func requestLog(request *http.Request) {
	log.Println("request: ", request.Method, request.URL.Path)
}

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Run local server for quick testing",
	Run: func(cmd *cobra.Command, args []string) {
		port, _ := cmd.Flags().GetInt("port")
		fmt.Println("Starting a simple http server on port: ", port)
		portDef := ":" + strconv.Itoa(port)
		healthHandler := func(w http.ResponseWriter, req *http.Request) {
			requestLog(req)
			io.WriteString(w, "healthy")
		}
		hostHandler := func(w http.ResponseWriter, req *http.Request) {
			host, _ := os.Hostname()
			requestLog(req)
			io.WriteString(w, host)
		}
		killSwitch := func(w http.ResponseWriter, req *http.Request) {
			requestLog(req)
			io.WriteString(w, "Killing web server")
			os.Exit(0)
		}
		http.HandleFunc("/health", healthHandler)
		http.HandleFunc("/host", hostHandler)
		http.HandleFunc("/kill", killSwitch)
		log.Fatal(http.ListenAndServe(portDef, nil))
	},
}
