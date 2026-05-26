package gui

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	. "modernc.org/tk9.0"
)

func (a *Controller) clearSyntaxHighlight() {
	for _, tag := range a.model.State.Diff.SyntaxTags {
		a.ui.DiffDetail.TagRemove(tag, "1.0", END)
	}
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
	a.ui.DiffDetail.TagConfigure(tag, Foreground(color))
	a.model.State.Diff.SyntaxTags[color] = tag
	return tag
}

func (*Controller) collectSyntaxSpans(
	lexer chroma.Lexer,
	style *chroma.Style,
	code string,
	lineNo,
	offset int,
	spans *[]syntaxSpan,
) {
	if lexer == nil || style == nil || code == "" {
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
			*spans = append(*spans, syntaxSpan{
				color:    color,
				line:     lineNo,
				startCol: col,
				endCol:   col + length,
			})
		}
		col += length
	}
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
