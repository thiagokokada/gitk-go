package gui

import (
	"runtime"

	. "modernc.org/tk9.0"
)

func diffDetailFontSpec() []any {
	if runtime.GOOS == "linux" {
		return []any{FixedFont}
	}
	return []any{CourierFont(), 11}
}
