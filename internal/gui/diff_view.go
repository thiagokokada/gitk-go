package gui

import (
	"fmt"

	"github.com/thiagokokada/gitk-go/internal/git"
)

const diffCommitSectionLabel = "Commit"

type diffViewModel struct {
	sections []git.FileSection
	rows     []diffViewRow
}

type diffViewRow struct {
	label    string
	addStart int
	addEnd   int
	delStart int
	delEnd   int
}

func newDiffViewModel(sections []git.FileSection) diffViewModel {
	if len(sections) == 0 {
		return diffViewModel{}
	}

	augmented := make([]git.FileSection, 0, len(sections)+1)
	augmented = append(augmented, summarizeDiffSections(sections))
	augmented = append(augmented, sections...)

	rows := make([]diffViewRow, 0, len(augmented))
	for _, sec := range augmented {
		rows = append(rows, diffSectionRow(sec))
	}

	return diffViewModel{
		sections: augmented,
		rows:     rows,
	}
}

func summarizeDiffSections(sections []git.FileSection) git.FileSection {
	total := git.FileSection{
		Path: diffCommitSectionLabel,
		Line: 1,
	}
	for _, section := range sections {
		total.Added += section.Added
		total.Removed += section.Removed
	}
	return total
}

func diffSectionRow(section git.FileSection) diffViewRow {
	label := section.Path + formatDiffSectionStats(section)
	addText := fmt.Sprintf("+%d", section.Added)
	delText := fmt.Sprintf("-%d", section.Removed)
	addStart := len(section.Path) + 2
	addEnd := addStart + len(addText)
	delStart := addEnd + 1
	delEnd := delStart + len(delText)
	return diffViewRow{
		label:    label,
		addStart: addStart,
		addEnd:   addEnd,
		delStart: delStart,
		delEnd:   delEnd,
	}
}

func formatDiffSectionStats(section git.FileSection) string {
	return fmt.Sprintf(" (+%d -%d)", section.Added, section.Removed)
}

func diffSectionLine(sections []git.FileSection, idx int) (int, bool) {
	if idx < 0 || idx >= len(sections) {
		return 0, false
	}
	line := sections[idx].Line
	if line <= 0 {
		return 0, false
	}
	return line, true
}

func diffSectionIndexForLine(sections []git.FileSection, line int) (int, bool) {
	if len(sections) == 0 {
		return 0, false
	}
	if line <= 0 {
		return 0, true
	}
	return fileSectionIndexForLine(sections, line), true
}

func diffFilePathForIndex(sections []git.FileSection, idx int) (string, bool) {
	if idx < 0 || idx >= len(sections) {
		return "", false
	}
	section := sections[idx]
	if section.Path == "" {
		return "", false
	}
	if idx == 0 && section.Path == diffCommitSectionLabel && section.Line == 1 {
		return "", false
	}
	return section.Path, true
}

func diffScrollFraction(line, totalLines int) float64 {
	if line <= 0 || totalLines <= 1 {
		return 0
	}
	fraction := float64(line-1) / float64(totalLines-1)
	if fraction < 0 {
		return 0
	}
	if fraction > 1 {
		return 1
	}
	return fraction
}
