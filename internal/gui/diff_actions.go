package gui

import (
	"fmt"
	"log/slog"

	. "modernc.org/tk9.0"

	"github.com/thiagokokada/gitk-go/internal/gui/tkutil"
)

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
	if len(chunks) == 0 {
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
			a.resetLocalDiffState(false)
			a.resetLocalDiffState(true)
			if stagedSelection, ok := a.state.selection.LocalSelection(); ok {
				a.renderLocalChanges(stagedSelection, true)
			}
			a.refreshLocalChangesAsync(true)
			a.setStatus(actionLabel + " done.")
		}, false)
	}(patch, reverse, actionLabel)
}
