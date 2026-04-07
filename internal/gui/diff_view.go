package gui

import "github.com/thiagokokada/gitk-go/internal/git"

const diffCommitSectionLabel = "Commit"

type diffViewModel struct {
	sections []git.FileSection
	labels   []string
}

func newDiffViewModel(sections []git.FileSection) diffViewModel {
	if len(sections) == 0 {
		return diffViewModel{}
	}

	augmented := make([]git.FileSection, 0, len(sections)+1)
	augmented = append(augmented, git.FileSection{Path: diffCommitSectionLabel, Line: 1})
	augmented = append(augmented, sections...)

	labels := make([]string, 0, len(augmented))
	for _, sec := range augmented {
		labels = append(labels, sec.Path)
	}

	return diffViewModel{
		sections: augmented,
		labels:   labels,
	}
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
