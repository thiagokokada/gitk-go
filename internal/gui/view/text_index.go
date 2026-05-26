package view

import (
	"fmt"
	"strconv"
	"strings"
)

func TextIndexLineNumber(index string) (line int, ok bool) {
	parts := strings.SplitN(index, ".", 2)
	if len(parts) == 0 {
		return 0, false
	}
	line, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, false
	}
	return line, true
}

func textIndex(line int, column int) string {
	return fmt.Sprintf("%d.%d", line, column)
}

func textEndIndex(line int) string {
	return fmt.Sprintf("%d.end", line)
}
