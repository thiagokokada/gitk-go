package gui

import (
	"fmt"
	"log/slog"
	"strings"
	"sync/atomic"

	"github.com/alecthomas/chroma/v2"
	"github.com/thiagokokada/gitk-go/internal/git"
	. "modernc.org/tk9.0"
)

const syntaxHighlightBatchSize = 400

type syntaxSpan struct {
	color    string
	line     int
	startCol int
	endCol   int
}

func (a *Controller) maybeStartSyntaxHighlight(content string, highlightDiff bool) {
	if !a.cfg.syntaxHighlight || !highlightDiff || content == "" {
		a.cancelSyntaxHighlight()
		a.clearSyntaxHighlight()
		return
	}
	metrics := syntaxHighlightMetricsFor(content)
	if ok, reason := shouldSyntaxHighlight(metrics); !ok {
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
	gen := atomic.AddUint64(&a.model.state.diff.syntaxGeneration, 1)
	go a.computeSyntaxHighlight(content, style, gen)
}

func (a *Controller) cancelSyntaxHighlight() {
	atomic.AddUint64(&a.model.state.diff.syntaxGeneration, 1)
}

func (a *Controller) syntaxHighlightCanceled(gen uint64) bool {
	return gen != atomic.LoadUint64(&a.model.state.diff.syntaxGeneration)
}

func (a *Controller) computeSyntaxHighlight(content string, style *chroma.Style, gen uint64) {
	lines := strings.Split(content, "\n")
	var currentLexer chroma.Lexer
	skipFile := false
	currentPath := ""
	batch := make([]syntaxSpan, 0, syntaxHighlightBatchSize)
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
				currentLexer = lexerForPath(path)
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
		if !shouldHighlightCodeLine(code) {
			skipFile = true
			reason := fmt.Sprintf(
				"line length exceeds %d char limit",
				maxSyntaxHighlightLineLength,
			)
			slog.Debug(
				"syntax highlight skipped for file",
				slog.String("path", currentPath),
				slog.String("reason", reason),
			)
			continue
		}
		a.collectSyntaxSpans(currentLexer, style, code, lineNo, offset, &batch)
		if len(batch) >= syntaxHighlightBatchSize {
			a.enqueueSyntaxSpans(batch, gen)
			batch = batch[:0]
		}
	}
	if len(batch) > 0 {
		a.enqueueSyntaxSpans(batch, gen)
	}
}

func (a *Controller) enqueueSyntaxSpans(spans []syntaxSpan, gen uint64) {
	if len(spans) == 0 {
		return
	}
	batch := append([]syntaxSpan(nil), spans...)
	PostEvent(func() {
		if a.syntaxHighlightCanceled(gen) {
			return
		}
		a.applySyntaxSpans(batch)
	}, false)
}

func (a *Controller) applySyntaxSpans(spans []syntaxSpan) {
	if a.ui.diffDetail == nil {
		return
	}
	for _, span := range spans {
		if span.color == "" {
			continue
		}
		tag := a.syntaxTagForColor(span.color)
		if tag == "" {
			continue
		}
		start := fmt.Sprintf("%d.%d", span.line, span.startCol)
		end := fmt.Sprintf("%d.%d", span.line, span.endCol)
		a.ui.diffDetail.TagAdd(tag, start, end)
	}
}
