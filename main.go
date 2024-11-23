package main

import (
	"github.com/himasagaratluri/netirk/cmd"
	"github.com/himasagaratluri/netirk/cmd/helpers"
)

func main() {
	helpers.GreetBanner()
	cmd.Execute()
}
