package view

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

const (
	MaxSyntaxHighlightBytes      = 1 * 1024 * 1024
	MaxSyntaxHighlightLineLength = 1024
)

type SyntaxSpan struct {
	Color    string
	Line     int
	StartCol int
	EndCol   int
}

type SyntaxHighlightMetrics struct {
	TotalBytes int
}

func SyntaxHighlightMetricsFor(content string) SyntaxHighlightMetrics {
	return SyntaxHighlightMetrics{
		TotalBytes: len(content),
	}
}

func ShouldSyntaxHighlight(metrics SyntaxHighlightMetrics) (ok bool, reason string) {
	if metrics.TotalBytes > MaxSyntaxHighlightBytes {
		return false, fmt.Sprintf(
			"diff size %d bytes exceeds %d byte limit",
			metrics.TotalBytes,
			MaxSyntaxHighlightBytes,
		)
	}
	return true, ""
}

func ShouldHighlightCodeLine(code string) bool {
	return len(code) <= MaxSyntaxHighlightLineLength
}

func CollectSyntaxSpans(
	lexer chroma.Lexer,
	style *chroma.Style,
	code string,
	lineNo int,
	offset int,
	spans *[]SyntaxSpan,
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
		color := ColorFromChromaEntry(entry)
		if color != "" {
			*spans = append(*spans, SyntaxSpan{
				Color:    color,
				Line:     lineNo,
				StartCol: col,
				EndCol:   col + length,
			})
		}
		col += length
	}
}

func ColorFromChromaEntry(entry chroma.StyleEntry) string {
	if entry.Colour.IsSet() {
		col := entry.Colour.String()
		col = strings.TrimPrefix(strings.ToLower(col), "#")
		return "#" + col
	}
	return ""
}

func LexerForPath(path string) chroma.Lexer {
	if path == "" {
		return nil
	}
	lexer := lexers.Match(path)
	if lexer == nil {
		lexer = lexers.Fallback
	}
	return chroma.Coalesce(lexer)
}
