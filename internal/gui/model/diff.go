package model

import (
	"fmt"
	"strings"

	"github.com/thiagokokada/gitk-go/internal/git"
)

func FileSectionIndexForLine(sections []git.FileSection, line int) int {
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

func PrepareDiffDisplay(content string, sections []git.FileSection) (string, []git.FileSection) {
	if content == "" {
		return content, sections
	}
	lines := strings.Split(content, "\n")
	var b strings.Builder
	newSections := make([]git.FileSection, len(sections))
	copy(newSections, sections)
	extraLines := 0
	nextSection := 0
	for i, line := range lines {
		lineNo := i + 1
		for nextSection < len(newSections) && newSections[nextSection].Line == lineNo {
			newSections[nextSection].Line = lineNo + extraLines
			nextSection++
		}
		if strings.HasPrefix(line, "diff --git ") && b.Len() > 0 {
			b.WriteString("\n")
			extraLines++
		}
		b.WriteString(line)
		if i < len(lines)-1 {
			b.WriteString("\n")
		}
	}
	for nextSection < len(newSections) {
		newSections[nextSection].Line += extraLines
		nextSection++
	}
	return b.String(), newSections
}

type DiffViewModel struct {
	Sections []git.FileSection
	Rows     []DiffViewRow
}

type DiffViewRow struct {
	Label    string
	AddStart int
	AddEnd   int
	DelStart int
	DelEnd   int
}

func NewDiffViewModel(sections []git.FileSection) DiffViewModel {
	if len(sections) == 0 {
		return DiffViewModel{}
	}

	augmented := make([]git.FileSection, 0, len(sections)+1)
	augmented = append(augmented, SummarizeDiffSections(sections))
	augmented = append(augmented, sections...)

	rows := make([]DiffViewRow, 0, len(augmented))
	for _, sec := range augmented {
		rows = append(rows, DiffSectionRow(sec))
	}

	return DiffViewModel{Sections: augmented, Rows: rows}
}

func SummarizeDiffSections(sections []git.FileSection) git.FileSection {
	total := git.FileSection{Path: DiffCommitSectionLabel, Line: 1}
	for _, section := range sections {
		total.Added += section.Added
		total.Removed += section.Removed
	}
	return total
}

func DiffSectionRow(section git.FileSection) DiffViewRow {
	label := section.Path + FormatDiffSectionStats(section)
	addText := fmt.Sprintf("+%d", section.Added)
	delText := fmt.Sprintf("-%d", section.Removed)
	addStart := len(section.Path) + 2
	addEnd := addStart + len(addText)
	delStart := addEnd + 1
	delEnd := delStart + len(delText)
	return DiffViewRow{
		Label:    label,
		AddStart: addStart,
		AddEnd:   addEnd,
		DelStart: delStart,
		DelEnd:   delEnd,
	}
}

func FormatDiffSectionStats(section git.FileSection) string {
	return fmt.Sprintf(" (+%d -%d)", section.Added, section.Removed)
}

func DiffSectionLine(sections []git.FileSection, idx int) (int, bool) {
	if idx < 0 || idx >= len(sections) {
		return 0, false
	}
	line := sections[idx].Line
	if line <= 0 {
		return 0, false
	}
	return line, true
}

func DiffSectionIndexForLine(sections []git.FileSection, line int) (int, bool) {
	if len(sections) == 0 {
		return 0, false
	}
	if line <= 0 {
		return 0, true
	}
	return FileSectionIndexForLine(sections, line), true
}

func DiffFilePathForIndex(sections []git.FileSection, idx int) (string, bool) {
	if idx < 0 || idx >= len(sections) {
		return "", false
	}
	section := sections[idx]
	if section.Path == "" {
		return "", false
	}
	if idx == 0 && section.Path == DiffCommitSectionLabel && section.Line == 1 {
		return "", false
	}
	return section.Path, true
}

func DiffScrollFraction(line, totalLines int) float64 {
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
