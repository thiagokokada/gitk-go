package cmd

import (
	"flag"
	"os"

	"github.com/thiagokokada/gitk-go/internal/git"
	"github.com/thiagokokada/gitk-go/internal/gui"
)

func Run() error {
	return run(os.Args[1:])
}

func run(args []string) error {
	fs := flag.NewFlagSet("gitk-go", flag.ContinueOnError)
	limit := fs.Int("limit", git.DefaultBatch, "number of commits to load per batch")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	repoPath := "."
	remaining := fs.Args()
	if len(remaining) > 0 {
		repoPath = remaining[len(remaining)-1]
	}
	return gui.Run(repoPath, *limit)
}
