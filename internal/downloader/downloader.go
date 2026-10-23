package downloader

import (
	"fmt"
	"strings"
)

// FormatBytes converts raw bytes into a human-readable string (e.g. 512 B, 1.4 MB)
func FormatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

// FormatSpeed returns a human-readable download rate (e.g. 14.2 MB/s)
func FormatSpeed(bytesPerSec float64) string {
	if bytesPerSec <= 0 {
		return "0 B/s"
	}
	return FormatBytes(int64(bytesPerSec)) + "/s"
}

// RenderProgressBar generates an ASCII progress bar (e.g. [=========>     ])
func RenderProgressBar(current, total, width int) string {
	if total <= 0 || width <= 0 {
		return "[" + strings.Repeat(" ", width) + "]"
	}
	filled := (current * width) / total
	if filled >= width {
		return "[" + strings.Repeat("=", width) + "]"
	}
	if filled == 0 {
		if current > 0 {
			return "[>" + strings.Repeat(" ", width-1) + "]"
		}
		return "[" + strings.Repeat(" ", width) + "]"
	}
	return "[" + strings.Repeat("=", filled-1) + ">" + strings.Repeat(" ", width-filled) + "]"
}
