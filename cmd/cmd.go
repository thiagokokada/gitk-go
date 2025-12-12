package cmd

import (
	"flag"
	"fmt"
	"os"
	"runtime/debug"

	"github.com/thiagokokada/gitk-go/internal/git"
	"github.com/thiagokokada/gitk-go/internal/gui"
)

func Run() error {
	return run(os.Args[1:])
}

func run(args []string) error {
	fs := flag.NewFlagSet("gitk-go", flag.ContinueOnError)
	limit := fs.Int("limit", git.DefaultBatch, "number of commits to load per batch")
	mode := fs.String("mode", gui.ThemeAuto.String(), "color mode: auto, light, or dark")
	watch := fs.Bool("watch", true, "automatically reload commits when repository changes")
	verbose := fs.Bool("verbose", false, "enable verbose logging")
	showVersion := fs.Bool("version", false, "print version information and exit")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	if *showVersion {
		fmt.Println(formatVersion())
		return nil
	}
	repoPath := "."
	remaining := fs.Args()
	if len(remaining) > 0 {
		repoPath = remaining[len(remaining)-1]
	}
	return gui.Run(repoPath, *limit, gui.ThemePreferenceFromString(*mode), *watch, *verbose)
}

func formatVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok || info == nil {
		return "dev"
	}
	version := info.Main.Version
	if version == "" || version == "(devel)" {
		version = "dev"
	}
	tags := buildSetting(info, "-tags")
	if tags == "" {
		return version
	}
	return fmt.Sprintf("%s (tags: %s)", version, tags)
}

func buildSetting(info *debug.BuildInfo, key string) string {
	for _, setting := range info.Settings {
		if setting.Key == key {
			return setting.Value
		}
	}
	return ""
}
