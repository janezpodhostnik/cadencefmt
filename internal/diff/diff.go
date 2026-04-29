package diff

import (
	"fmt"
	"strings"
)

// Unified returns a unified diff between a and b, labeled with the given filename.
func Unified(filename string, a, b string) string {
	aLines := strings.Split(a, "\n")
	bLines := strings.Split(b, "\n")

	var out strings.Builder
	fmt.Fprintf(&out, "--- %s\n", filename)
	fmt.Fprintf(&out, "+++ %s\n", filename)

	// Simple diff: show all lines that differ
	maxLen := len(aLines)
	if len(bLines) > maxLen {
		maxLen = len(bLines)
	}

	// Find first and last differing lines
	firstDiff := -1
	lastDiff := -1
	for i := 0; i < maxLen; i++ {
		la, lb := "", ""
		if i < len(aLines) {
			la = aLines[i]
		}
		if i < len(bLines) {
			lb = bLines[i]
		}
		if la != lb {
			if firstDiff == -1 {
				firstDiff = i
			}
			lastDiff = i
		}
	}

	if firstDiff == -1 {
		return "" // no differences
	}

	// Show context around diffs
	contextLines := 3
	start := firstDiff - contextLines
	if start < 0 {
		start = 0
	}
	end := lastDiff + contextLines + 1
	if end > maxLen {
		end = maxLen
	}

	aCount := 0
	bCount := 0
	var hunks strings.Builder

	for i := start; i < end; i++ {
		la, lb := "", ""
		if i < len(aLines) {
			la = aLines[i]
		}
		if i < len(bLines) {
			lb = bLines[i]
		}

		if la == lb {
			fmt.Fprintf(&hunks, " %s\n", la)
			aCount++
			bCount++
		} else {
			if i < len(aLines) {
				fmt.Fprintf(&hunks, "-%s\n", la)
				aCount++
			}
			if i < len(bLines) {
				fmt.Fprintf(&hunks, "+%s\n", lb)
				bCount++
			}
		}
	}

	fmt.Fprintf(&out, "@@ -%d,%d +%d,%d @@\n", start+1, aCount, start+1, bCount)
	out.WriteString(hunks.String())
	return out.String()
}
