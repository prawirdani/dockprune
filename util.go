package main

import (
	"fmt"
	"time"
)

// relTime renders a unix-seconds timestamp as a short "5h ago" style string.
func relTime(sec int64) string {
	if sec <= 0 {
		return "unknown"
	}
	d := time.Since(time.Unix(sec, 0))
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}

// humanSize renders a byte count as a compact human-readable string.
// Docker reports sizes in decimal units, so we do too.
func humanSize(b int64) string {
	if b <= 0 {
		return "-"
	}
	const unit = 1000
	if b < unit {
		return fmt.Sprintf("%dB", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%cB", float64(b)/float64(div), "kMGTPE"[exp])
}

// truncate shortens s to max runes, adding an ellipsis. Rune-safe.
func truncate(s string, max int) string {
	if max <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if max == 1 {
		return "…"
	}
	return string(r[:max-1]) + "…"
}

// oneLine collapses a multi-line string into its first line for compact display.
func oneLine(s string) string {
	for i, c := range s {
		if c == '\n' {
			return s[:i]
		}
	}
	return s
}
