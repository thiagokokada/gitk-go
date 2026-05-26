package gui

import (
	"strings"

	"github.com/thiagokokada/gitk-go/internal/gui/model"
	"github.com/thiagokokada/gitk-go/internal/gui/selection"
)

func selectionMatchesTreeID(sel *selection.State, id string) bool {
	id = strings.TrimSpace(id)
	if id == "" {
		return false
	}
	if staged, ok := sel.LocalSelection(); ok {
		return id == model.LocalRowID(staged)
	}
	hash := sel.CommitHash()
	return hash != "" && hash == id
}
