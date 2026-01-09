package main

import (
	"log"

	"github.com/thiagokokada/gitk-go/cmd"

	. "modernc.org/tk9.0"
)

func main() {
	// tk9.0 requires ActivateTheme to be invoked from the main package.
	cmd.SetThemeActivator(func(name string) error {
		return ActivateTheme(name)
	})
	if err := cmd.Run(); err != nil {
		log.Fatalf("gitk-go: %v", err)
	}
}
