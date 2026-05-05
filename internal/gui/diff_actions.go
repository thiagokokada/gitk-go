package gui

import (
	"fmt"
	"log/slog"
	"strings"

	. "modernc.org/tk9.0"

	"github.com/thiagokokada/gitk-go/internal/gui/tkutil"
)

const maxInlineDiffActions = 64

func (a *Controller) clearInlineDiffActions() {
	for _, btn := range a.state.diff.inlineButtons {
		Destroy(btn)
	}
	a.state.diff.inlineButtons = nil
}

func (a *Controller) addInlineDiffActions(staged bool, diff string) {
	if a.ui.diffDetail == nil {
		return
	}
	chunks := parseDiffChunks(diff)
	if !shouldAddInlineDiffActions(diff, chunks) {
		return
	}
	label := "+"
	actionPrefix := "Staging"
	reverse := false
	if staged {
		label = "-"
		actionPrefix = "Unstaging"
		reverse = true
	}
	a.ui.diffDetail.Configure(State(NORMAL))
	defer a.ui.diffDetail.Configure(State("disabled"))
	for _, chunk := range chunks {
		if patch, ok := buildFilePatch(chunk); ok {
			a.insertDiffActionButton(chunk.lineNo, label, patch, reverse, actionPrefix+" file")
		}
		for _, hunk := range chunk.hunks {
			if patch, ok := buildHunkPatch(chunk, hunk); ok {
				a.insertDiffActionButton(hunk.lineNo, label, patch, reverse, actionPrefix+" hunk")
			}
		}
	}
}

func shouldAddInlineDiffActions(diff string, chunks []diffFileChunk) bool {
	if len(chunks) == 0 || containsUnmergedPathMarker(diff) {
		return false
	}
	total := 0
	for _, chunk := range chunks {
		total++
		total += len(chunk.hunks)
		if total > maxInlineDiffActions {
			return false
		}
	}
	return true
}

func containsUnmergedPathMarker(diff string) bool {
	for line := range strings.SplitSeq(diff, "\n") {
		if strings.HasPrefix(line, "* Unmerged path ") {
			return true
		}
	}
	return false
}

func (a *Controller) insertDiffActionButton(lineNo int, label string, patch string, reverse bool, actionLabel string) {
	if lineNo <= 0 || patch == "" {
		return
	}
	btn := App.TButton(
		Txt(label),
		Width(2),
		Padding("1p"),
		Command(func() {
			a.applyDiffPatch(patch, reverse, actionLabel)
		}),
	)
	a.state.diff.inlineButtons = append(a.state.diff.inlineButtons, btn)
	index := fmt.Sprintf("%d.0", lineNo)
	if _, err := tkutil.Evalf("%s window create %s -window %s -padx 2 -pady 0", a.ui.diffDetail, index, btn); err != nil {
		slog.Debug("diff action button", slog.Any("error", err))
	}
}

func (a *Controller) applyDiffPatch(patch string, reverse bool, actionLabel string) {
	if a.svc == nil {
		return
	}
	a.setStatus(actionLabel + "...")
	go func(patch string, reverse bool, actionLabel string) {
		err := a.svc.StagePatchToIndex(patch, reverse)
		PostEvent(func() {
			if err != nil {
				a.setStatus(fmt.Sprintf("%s failed: %v", actionLabel, err))
				return
			}
			if stagedSelection, ok := a.state.selection.LocalSelection(); ok {
				if a.optimisticRemoveDiffPatch(stagedSelection, patch) {
					a.refreshLocalChangesAsync(true)
					a.setStatus(actionLabel + " done.")
					return
				}
				a.resetLocalDiffState(false)
				a.resetLocalDiffState(true)
				a.renderLocalChanges(stagedSelection, true)
			}
			a.refreshLocalChangesAsync(true)
			a.setStatus(actionLabel + " done.")
		}, false)
	}(patch, reverse, actionLabel)
}

func (a *Controller) optimisticRemoveDiffPatch(staged bool, patch string) bool {
	if a.ui.diffDetail == nil {
		return false
	}
	diffText := a.ui.diffDetail.Get("1.0", "end-1c")[0]
	displayText, sections, ok := removePatchFromDiffText(diffText, patch)
	if !ok {
		return false
	}
	topLine := a.diffTopLine()
	a.writeDetailText(displayText, len(sections) > 0)
	if topLine > 0 {
		a.scrollDiffToLine(topLine)
	}
	a.setFileSections(sections)
	a.addInlineDiffActions(staged, displayText)
	return true
}
