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
	limit := fs.Int("limit", git.DefaultBatch, "number of commits to load per batch (larger uses more CPU/memory)")
	graphCols := fs.Int("graph-cols", git.DefaultGraphMaxColumns, "max number of graph columns to render (lower uses less CPU/memory)")
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
		return nil
	}
	repoPath := "."
	remaining := fs.Args()
	if len(remaining) > 0 {
		repoPath = remaining[len(remaining)-1]
	}
	return gui.Run(repoPath, *limit, *graphCols, !*textGraph, gui.ThemePreferenceFromString(*mode), !*noWatch, !*noSyntax, *verbose)
}
