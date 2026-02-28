package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// dimBackground applies the ANSI faint/dim attribute to an entire rendered
// string so it appears subdued behind a modal dialog. It re-inserts the dim
// code after every SGR reset so that lipgloss style boundaries don't cancel it.
func dimBackground(s string) string {
	const dim = "\x1b[2m"
	s = strings.ReplaceAll(s, "\x1b[0m", "\x1b[0m"+dim)
	return dim + s + "\x1b[0m"
}

// placeOverlay renders fg centered on top of bg.
// bgW and bgH are the terminal dimensions used for centering.
func placeOverlay(fg, bg string, bgW, bgH int) string {
	fgLines := strings.Split(fg, "\n")
	bgLines := strings.Split(bg, "\n")

	// Measure dialog dimensions.
	fgW := 0
	for _, l := range fgLines {
		if w := lipgloss.Width(l); w > fgW {
			fgW = w
		}
	}
	fgH := len(fgLines)

	// Center position.
	startX := (bgW - fgW) / 2
	startY := (bgH - fgH) / 2
	if startX < 0 {
		startX = 0
	}
	if startY < 0 {
		startY = 0
	}

	for i, fgLine := range fgLines {
		bgIdx := startY + i
		if bgIdx >= len(bgLines) {
			break
		}
		bgLine := bgLines[bgIdx]
		fgLineW := lipgloss.Width(fgLine)

		left := ansi.Truncate(bgLine, startX, "")
		// Pad with spaces if the background line is shorter than startX.
		if lw := lipgloss.Width(left); lw < startX {
			left += strings.Repeat(" ", startX-lw)
		}
		right := ansi.TruncateLeft(bgLine, startX+fgLineW, "")

		bgLines[bgIdx] = left + fgLine + right
	}

	return strings.Join(bgLines, "\n")
}
