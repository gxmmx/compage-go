package console

import (
	"fmt"
	"io"
	"strings"
)

const columnPadding = 2

// renderTable writes auto-width columnar output to w.
// Headers and rows are left-aligned with 2-space padding between columns.
// If rows is empty, nothing is written.
func renderTable(w io.Writer, headers []string, rows [][]string) {
	if len(rows) == 0 {
		return
	}

	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) && len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}

	writeLine(w, headers, widths)
	for _, row := range rows {
		writeLine(w, row, widths)
	}
}

func writeLine(w io.Writer, cells []string, widths []int) {
	var b strings.Builder
	for i, cell := range cells {
		if i == len(cells)-1 {
			b.WriteString(cell)
		} else {
			pad := widths[i] - len(cell) + columnPadding
			b.WriteString(cell)
			b.WriteString(strings.Repeat(" ", pad))
		}
	}
	_, _ = fmt.Fprintln(w, b.String())
}
