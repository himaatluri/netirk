package helpers

import (
	"fmt"
)

const AppName string = `| \ | | ___| |_(_)_ __| | __
|  \| |/ _ \ __| | '__| |/ /
| |\  |  __/ |_| | |  |   < 
|_| \_|\___|\__|_|_|  |_|\_\`

func GreetBanner() {
	fmt.Print("\n", AppName, "\n\n")
}
