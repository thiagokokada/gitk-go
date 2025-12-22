package gui

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	. "modernc.org/tk9.0"
)

func (a *Controller) applySyntaxHighlight(content string) {
	if a.ui.diffDetail == nil || content == "" {
		return
	}
	a.clearSyntaxHighlight()
	style := styleForPalette(a.theme.palette)
	if style == nil {
		return
	}
	lines := strings.Split(content, "\n")
	var currentLexer chroma.Lexer
	for i, line := range lines {
		lineNo := i + 1
		if path, ok := diffPathFromLine(line); ok {
			currentLexer = nil
			if path != "" {
				currentLexer = lexerForPath(path)
			}
			continue
		}
		if strings.HasPrefix(line, "--- ") || strings.HasPrefix(line, "+++ ") || strings.HasPrefix(line, "@@") {
			continue
		}
		if currentLexer == nil {
			continue
		}
		code, offset, ok := diffLineCode(line)
		if !ok {
			continue
		}
		a.highlightCodeLine(currentLexer, style, code, lineNo, offset)
	}
}

func (a *Controller) clearSyntaxHighlight() {
	if a.ui.diffDetail == nil {
		return
	}
	for _, tag := range a.state.diff.syntaxTags {
		a.ui.diffDetail.TagRemove(tag, "1.0", END)
	}
}

func (a *Controller) syntaxTagForColor(color string) string {
	if color == "" || a.ui.diffDetail == nil {
		return ""
	}
	if a.state.diff.syntaxTags == nil {
		a.state.diff.syntaxTags = make(map[string]string)
	}
	if tag, ok := a.state.diff.syntaxTags[color]; ok {
		return tag
	}
	tag := fmt.Sprintf("syntax_%d", len(a.state.diff.syntaxTags))
	a.ui.diffDetail.TagConfigure(tag, Foreground(color))
	a.state.diff.syntaxTags[color] = tag
	return tag
}

func (a *Controller) highlightCodeLine(lexer chroma.Lexer, style *chroma.Style, code string, lineNo, offset int) {
	if a.ui.diffDetail == nil || lexer == nil || style == nil || code == "" {
		return
	}
	iterator, err := lexer.Tokenise(nil, code)
	if err != nil {
		return
	}
	col := offset
	for _, token := range iterator.Tokens() {
		value := token.Value
		if value == "" {
			continue
		}
		length := utf8.RuneCountInString(value)
		entry := style.Get(token.Type)
		color := colorFromEntry(entry)
		if color != "" {
			tag := a.syntaxTagForColor(color)
			if tag != "" {
				start := fmt.Sprintf("%d.%d", lineNo, col)
				end := fmt.Sprintf("%d.%d", lineNo, col+length)
				a.ui.diffDetail.TagAdd(tag, start, end)
			}
		}
		col += length
	}
}

func styleForPalette(p colorPalette) *chroma.Style {
	if p.isDark() {
		if st := styles.Get("github-dark"); st != nil {
			return st
		}
	} else {
		if st := styles.Get("github"); st != nil {
			return st
		}
	}
	return styles.Fallback
}

func colorFromEntry(entry chroma.StyleEntry) string {
	if entry.Colour.IsSet() {
		col := entry.Colour.String()
		col = strings.TrimPrefix(strings.ToLower(col), "#")
		return "#" + col
	}
	return ""
}

func lexerForPath(path string) chroma.Lexer {
	if path == "" {
		return nil
	}
	lexer := lexers.Match(path)
	if lexer == nil {
		lexer = lexers.Fallback
	}
	return chroma.Coalesce(lexer)
}
