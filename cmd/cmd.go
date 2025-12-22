package cmd

import (
	"flag"
	"fmt"
	"os"

	"github.com/thiagokokada/gitk-go/internal/buildinfo"
	"github.com/thiagokokada/gitk-go/internal/git"
	"github.com/thiagokokada/gitk-go/internal/gui"
)

func Run() error {
	return run(os.Args[1:])
}

func run(args []string) error {
	fs := flag.NewFlagSet("gitk-go", flag.ContinueOnError)
	limit := fs.Uint("limit", uint(git.DefaultBatch), "number of commits to load per batch (larger uses more CPU/memory)")
	graphCols := fs.Uint("graph-cols", uint(git.DefaultGraphMaxColumns), "max number of graph columns to render (lower uses less CPU/memory)")
	textGraph := fs.Bool("text-graph", false, "render commit graph as text (disables canvas graph)")
	mode := fs.String("mode", gui.ThemeAuto.String(), "color mode: auto, light, or dark")
	noWatch := fs.Bool("nowatch", false, "disable automatic reload when repository changes")
	noSyntax := fs.Bool("nosyntax", false, "disable syntax highlighting in the diff viewer")
	verbose := fs.Bool("verbose", false, "enable verbose logging")
	showVersion := fs.Bool("version", false, "print version information and exit")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	if *showVersion {
		fmt.Println(buildinfo.VersionWithTags())
		if gitVer, err := git.GitVersion(); gitVer != "" {
			fmt.Println(gitVer)
			if err != nil {
				fmt.Fprintf(os.Stderr, "git version warning: %v\n", err)
			}
		} else if err != nil {
			fmt.Fprintf(os.Stderr, "git version unavailable: %v\n", err)
		}
		return nil
	}
	limitU := *limit
	if limitU == 0 {
		limitU = git.DefaultBatch
	}
	graphColsU := *graphCols
	if graphColsU == 0 {
		graphColsU = git.DefaultGraphMaxColumns
	}
	repoPath := "."
	remaining := fs.Args()
	if len(remaining) > 0 {
		repoPath = remaining[len(remaining)-1]
	}
	return gui.Run(repoPath, limitU, graphColsU, !*textGraph, gui.ThemePreferenceFromString(*mode), !*noWatch, !*noSyntax, *verbose)
}
