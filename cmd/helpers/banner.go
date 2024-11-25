package helpers

import (
	"fmt"
)

const AppName string = `               _    _        _    
  _ __    ___ | |_ (_) _ __ | | __
 | '_ \  / _ \| __|| || '__|| |/ /
 | | | ||  __/| |_ | || |   |   < 
 |_| |_| \___| \__||_||_|   |_|\_\
                                  `

func GreetBanner() {
	fmt.Print("\n", AppName, "\n\n")
}
