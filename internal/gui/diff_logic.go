package gui

import "github.com/thiagokokada/gitk-go/internal/git"

func fileSectionIndexForLine(sections []git.FileSection, line int) int {
	if len(sections) == 0 || line <= 0 {
		return 0
	}
	target := 0
	for i, sec := range sections {
		if line < sec.Line {
			break
		}
		target = i
	}
	return target
}
