package helpers

import (
	"fmt"
	"os"

	yaml "gopkg.in/yaml.v3"
)

type Targets struct {
	Targets []string `yaml:"targets,omitempty"`
}

func ParseTargetFile(path string) Targets {
	content, err := os.ReadFile(path)
	if err != nil {
		fmt.Println(err)
	}

	hostTargets := Targets{}

	yaml.Unmarshal(content, &hostTargets)

	return hostTargets
}
