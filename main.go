package main

import (
	"log"

	"github.com/thiagokokada/gitk-go/cmd"
)

func main() {
	if err := cmd.Run(); err != nil {
		log.Fatalf("gitk-go: %v", err)
	}
}
