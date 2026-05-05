package gui

import (
	"strconv"
	"strings"
)

func textIndexLineNumber(index string) (line int, ok bool) {
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
