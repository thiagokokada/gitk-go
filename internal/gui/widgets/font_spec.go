package widgets

import (
	"runtime"

	. "modernc.org/tk9.0"
)

func graphCanvasLabelFontSpec() []any {
	if runtime.GOOS == "linux" {
		return []any{DefaultFont}
	}
	return []any{DefaultFont, 9}
}
