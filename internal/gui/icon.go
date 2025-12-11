package gui

import (
	_ "embed"
	"strings"

	. "modernc.org/tk9.0"
)

//go:embed assets/appicon.svg
var appIconSVG string

func applyAppIcon() {
	if strings.TrimSpace(appIconSVG) == "" {
		return
	}
	img := NewPhoto(Data(appIconSVG))
	if img == nil {
		return
	}
	App.IconPhoto(img)
}
