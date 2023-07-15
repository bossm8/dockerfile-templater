// Dockerfile Templating Utility
// Generates Dockerfiles from a template with different variants read from
// yaml files

// Author: <bossm8@hotmail.com>

package main

import (
	"log"

	"github.com/bossm8/dockerfile-templater/cmd"
	"github.com/bossm8/dockerfile-templater/utils"
)

func main() {
	log.SetFlags(0)

	if err := cmd.TemplaterCMD.Execute(); err != nil {
		utils.Error("Failed to execute the templater: %s", err)
	}
}
