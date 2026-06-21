package model

import (
	"sync/atomic"

	"github.com/thiagokokada/gitk-go/internal/git"
)

type DiffState struct {
	SyntaxGeneration      atomic.Uint64
	FileSections          []git.FileSection
	SyntaxTags            map[string]string
	SelectedFileIndex     int
	SuppressFileSelection bool
	SkipNextSync          bool
}

func NewDiffState() DiffState {
	return DiffState{SyntaxTags: make(map[string]string), SelectedFileIndex: -1}
}

func (d *DiffState) SetFileSections(sections []git.FileSection) {
	d.FileSections = sections
	d.SelectedFileIndex = -1
}

func (d *DiffState) BeginUserFileSelection(idx int) (line int, ok bool) {
	if d.SuppressFileSelection {
		return 0, false
	}
	line, ok = DiffSectionLine(d.FileSections, idx)
	if !ok {
		return 0, false
	}
	d.SkipNextSync = true
	return line, true
}

func (d *DiffState) SyncSelectionIndexForLine(line int) (idx int, ok bool) {
	if line <= 0 {
		return 0, false
	}
	return DiffSectionIndexForLine(d.FileSections, line)
}

func (d *DiffState) ConsumeSkipNextSync() bool {
	if !d.SkipNextSync {
		return false
	}
	d.SkipNextSync = false
	return true
}

func (d *DiffState) SelectFileIndex(idx int) bool {
	if idx < 0 || idx >= len(d.FileSections) {
		return false
	}
	if d.SelectedFileIndex == idx {
		return false
	}
	d.SuppressFileSelection = true
	d.SelectedFileIndex = idx
	return true
}

func (d *DiffState) FinishProgrammaticFileSelection() {
	d.SuppressFileSelection = false
}

func (d *DiffState) SelectedFilePath() (string, bool) {
	return DiffFilePathForIndex(d.FileSections, d.SelectedFileIndex)
}

type DiffRequest struct {
	Entry git.Entry
	Hash  string
}
