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
	if a.diff.detail == nil || content == "" {
		return
	}
	a.clearSyntaxHighlight()
	style := styleForPalette(a.palette)
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
	if a.diff.detail == nil {
		return
	}
	for _, tag := range a.diff.syntaxTags {
		a.diff.detail.TagRemove(tag, "1.0", END)
	}
}

func (a *Controller) syntaxTagForColor(color string) string {
	if color == "" || a.diff.detail == nil {
		return ""
	}
	if a.diff.syntaxTags == nil {
		a.diff.syntaxTags = make(map[string]string)
	}
	if tag, ok := a.diff.syntaxTags[color]; ok {
		return tag
	}
	tag := fmt.Sprintf("syntax_%d", len(a.diff.syntaxTags))
	a.diff.detail.TagConfigure(tag, Foreground(color))
	a.diff.syntaxTags[color] = tag
	return tag
}

func (a *Controller) highlightCodeLine(lexer chroma.Lexer, style *chroma.Style, code string, lineNo, offset int) {
	if a.diff.detail == nil || lexer == nil || style == nil || code == "" {
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
				a.diff.detail.TagAdd(tag, start, end)
			}
		}
		col += length
	}
}

func styleForPalette(p colorPalette) *chroma.Style {
	if p.isDark() {
		if st := styles.Get("dracula"); st != nil {
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

func advancePosition(line, col int, text string) (int, int) {
	for _, r := range text {
		if r == '\n' {
			line++
			col = 0
			continue
		}
		col++
	}
	return line, col
}

func diffPathFromLine(line string) (string, bool) {
	const prefix = "diff --git "
	if !strings.HasPrefix(line, prefix) {
		return "", false
	}
	segment := strings.TrimSpace(line[len(prefix):])
	tokens := diffLineTokens(segment)
	if len(tokens) < 2 {
		return "", true
	}
	return normalizeDiffPath(tokens[1]), true
}

func diffLineTokens(s string) []string {
	var tokens []string
	for {
		s = strings.TrimLeft(s, " \t")
		if s == "" {
			break
		}
		if s[0] == '"' {
			var buf strings.Builder
			escaped := false
			i := 1
			for i < len(s) {
				ch := s[i]
				if escaped {
					buf.WriteByte(ch)
					escaped = false
					i++
					continue
				}
				if ch == '\\' {
					escaped = true
					i++
					continue
				}
				if ch == '"' {
					i++
					break
				}
				buf.WriteByte(ch)
				i++
			}
			tokens = append(tokens, buf.String())
			s = s[i:]
			continue
		}
		j := 0
		for j < len(s) && s[j] != ' ' && s[j] != '\t' {
			j++
		}
		tokens = append(tokens, s[:j])
		s = s[j:]
	}
	return tokens
}

func normalizeDiffPath(token string) string {
	token = strings.TrimPrefix(token, "a/")
	token = strings.TrimPrefix(token, "b/")
	return token
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

func diffLineCode(line string) (string, int, bool) {
	if line == "" {
		return "", 0, false
	}
	switch line[0] {
	case '+', '-', ' ':
		if strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---") {
			return "", 0, false
		}
		if strings.HasPrefix(line, "\\ ") {
			return "", 0, false
		}
		return line[1:], 1, true
	default:
		return "", 0, false
	}
}
