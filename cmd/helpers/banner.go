package helpers

import (
	"fmt"
	"log"
	"strings"
)

const AppName string = "| Netirk |"

var BannerLines = strings.Repeat("-", len(AppName))

func PrintAppBanner() {
	log.Print(BannerLines)
	log.Print(AppName)
	log.Print(BannerLines)
}

func GreetBanner() {
	fmt.Print("\n", BannerLines, "\n", AppName, "\n", BannerLines, "\n")
}
