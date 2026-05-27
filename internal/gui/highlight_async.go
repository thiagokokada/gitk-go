package gui

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/thiagokokada/gitk-go/internal/git"
	"github.com/thiagokokada/gitk-go/internal/gui/view"
	. "modernc.org/tk9.0"
)

const syntaxHighlightBatchSize = 400

func (a *Controller) maybeStartSyntaxHighlight(content string, highlightDiff bool) {
	if !a.cfg.syntaxHighlight || !highlightDiff || content == "" {
		a.cancelSyntaxHighlight()
		a.clearSyntaxHighlight()
		return
	}
	metrics := view.SyntaxHighlightMetricsFor(content)
	if ok, reason := view.ShouldSyntaxHighlight(metrics); !ok {
		if reason != "" {
			slog.Debug("syntax highlight skipped", slog.String("reason", reason))
		}
		a.cancelSyntaxHighlight()
		a.clearSyntaxHighlight()
		return
	}
	a.clearSyntaxHighlight()
	a.startSyntaxHighlight(content)
}

func (a *Controller) startSyntaxHighlight(content string) {
	style := a.theme.palette.chromaStyle()
	if style == nil {
		return
	}
	gen := a.model.State.Diff.SyntaxGeneration.Add(1)
	go a.computeSyntaxHighlight(content, style, gen)
}

func (a *Controller) cancelSyntaxHighlight() {
	a.model.State.Diff.SyntaxGeneration.Add(1)
}

func (a *Controller) syntaxHighlightCanceled(gen uint64) bool {
	return gen != a.model.State.Diff.SyntaxGeneration.Load()
}

func (a *Controller) computeSyntaxHighlight(content string, style *chroma.Style, gen uint64) {
	lines := strings.Split(content, "\n")
	var currentLexer chroma.Lexer
	skipFile := false
	currentPath := ""
	batch := make([]view.SyntaxSpan, 0, syntaxHighlightBatchSize)
	for i, line := range lines {
		if a.syntaxHighlightCanceled(gen) {
			return
		}
		lineNo := i + 1
		if path, ok := git.DiffPathFromLine(line); ok {
			currentLexer = nil
			skipFile = false
			currentPath = path
			if path != "" {
				currentLexer = view.LexerForPath(path)
			}
			continue
		}
		if strings.HasPrefix(line, "--- ") || strings.HasPrefix(line, "+++ ") || strings.HasPrefix(line, "@@") {
			continue
		}
		if currentLexer == nil || skipFile {
			continue
		}
		code, offset, ok := git.DiffLineCode(line)
		if !ok {
			continue
		}
		if !view.ShouldHighlightCodeLine(code) {
			skipFile = true
			reason := fmt.Sprintf(
				"line length exceeds %d char limit",
				view.MaxSyntaxHighlightLineLength,
			)
			slog.Debug(
				"syntax highlight skipped for file",
				slog.String("path", currentPath),
				slog.String("reason", reason),
			)
			continue
		}
		view.CollectSyntaxSpans(currentLexer, style, code, lineNo, offset, &batch)
		if len(batch) >= syntaxHighlightBatchSize {
			a.enqueueSyntaxSpans(batch, gen)
			batch = batch[:0]
		}
	}
	if len(batch) > 0 {
		a.enqueueSyntaxSpans(batch, gen)
	}
}

func (a *Controller) enqueueSyntaxSpans(spans []view.SyntaxSpan, gen uint64) {
	if len(spans) == 0 {
		return
	}
	batch := append([]view.SyntaxSpan(nil), spans...)
	PostEvent(func() {
		if a.syntaxHighlightCanceled(gen) {
			return
		}
		a.applySyntaxSpans(batch)
	}, false)
}

func (a *Controller) applySyntaxSpans(spans []view.SyntaxSpan) {
	for _, span := range spans {
		if span.Color == "" {
			continue
		}
		tag := a.syntaxTagForColor(span.Color)
		if tag == "" {
			continue
		}
		a.ui.ApplySyntaxSpan(tag, span.Line, span.StartCol, span.EndCol)
	}
}
