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

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Run local server for quick testing",
	Run: func(cmd *cobra.Command, args []string) {
		port, _ := cmd.Flags().GetInt("port")
		fmt.Printf("Starting a simple http server on port, %v", port)
		portDef := ":" + strconv.Itoa(port)
		healthHandler := func(w http.ResponseWriter, _ *http.Request) {
			io.WriteString(w, "healthy")
		}
		hostHandler := func(w http.ResponseWriter, _ *http.Request) {
			host, _ := os.Hostname()
			io.WriteString(w, host)
		}
		http.HandleFunc("/health", healthHandler)
		http.HandleFunc("/host", hostHandler)
		log.Fatal(http.ListenAndServe(portDef, nil))
	},
}
