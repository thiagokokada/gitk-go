//go:build nosyntaxhighlight

package gui

import "github.com/alecthomas/chroma/v2"

func (a *Controller) applySyntaxHighlight(content string) {}

func (a *Controller) clearSyntaxHighlight() {}

func (a *Controller) syntaxTagForColor(color string) string { return "" }

func styleForPalette(p colorPalette) *chroma.Style { return nil }
