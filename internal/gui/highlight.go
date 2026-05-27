package gui

import (
	"fmt"
)

func (a *Controller) clearSyntaxHighlight() {
	a.ui.ClearSyntaxTags(a.model.State.Diff.SyntaxTags)
}

func (a *Controller) syntaxTagForColor(color string) string {
	if color == "" {
		return ""
	}
	if a.model.State.Diff.SyntaxTags == nil {
		a.model.State.Diff.SyntaxTags = make(map[string]string)
	}
	if tag, ok := a.model.State.Diff.SyntaxTags[color]; ok {
		return tag
	}
	tag := fmt.Sprintf("syntax_%d", len(a.model.State.Diff.SyntaxTags))
	a.ui.ConfigureSyntaxTag(tag, color)
	a.model.State.Diff.SyntaxTags[color] = tag
	return tag
}
