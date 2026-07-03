package service

import (
	"fmt"
	"html"
	"regexp"
	"strings"
)

// markdownToOOXML converts a Markdown string into OOXML paragraph XML.
// Supported syntax:
//   - # / ## / ### / #### → Heading 1-4
//   - - or * prefix → bullet list item
//   - 1. / 2. prefix → numbered list item (rendered as plain paragraphs)
//   - **bold** → bold run
//   - *italic* → italic run
//   - `code` → monospace run
//   - [!figure:id type=X caption=Y] → figure placeholder paragraph
//   - blank lines → paragraph breaks
//   - | table | syntax → simple table rendering
//
// Unsupported constructs are emitted as normal paragraphs.
func markdownToOOXML(md string) string {
	var buf strings.Builder
	lines := strings.Split(md, "\n")

	inTable := false
	var tableRows [][]string

	flushTable := func() {
		if !inTable || len(tableRows) == 0 {
			return
		}
		buf.WriteString(renderTableXML(tableRows))
		tableRows = nil
		inTable = false
	}

	for _, raw := range lines {
		line := strings.TrimSpace(raw)

		// Table handling.
		if strings.HasPrefix(line, "|") && strings.HasSuffix(line, "|") {
			inTable = true
			cells := parseTableRow(line)
			// Skip separator rows (|---|---|).
			if !isTableSeparator(line) {
				tableRows = append(tableRows, cells)
			}
			continue
		}
		flushTable()

		// Skip empty lines.
		if line == "" {
			continue
		}

		// Figure placeholder.
		if strings.HasPrefix(line, "[!figure:") {
			buf.WriteString(renderFigurePlaceholder(line))
			continue
		}

		// Headings.
		if h := parseHeading(line); h != "" {
			buf.WriteString(h)
			continue
		}

		// Bullet list items.
		if isBulletItem(line) {
			text := stripBulletPrefix(line)
			buf.WriteString(renderBulletItem(text))
			continue
		}

		// Numbered list items.
		if num, text, ok := parseNumberedItem(line); ok {
			buf.WriteString(renderNumberedItem(num, text))
			continue
		}

		// Regular paragraph with inline formatting.
		buf.WriteString(renderParagraph(line))
	}
	flushTable()

	return buf.String()
}

// parseHeading returns the OOXML for a Markdown heading, or "" if the line
// is not a heading.
func parseHeading(line string) string {
	re := regexp.MustCompile(`^(#{1,4})\s+(.+)`)
	m := re.FindStringSubmatch(line)
	if m == nil {
		return ""
	}
	level := len(m[1])
	text := strings.TrimSpace(m[2])
	return fmt.Sprintf(`<w:p><w:pPr><w:pStyle w:val="Heading%d"/></w:pPr>%s</w:p>`,
		level, renderInlineRuns(text))
}

// isBulletItem returns true if the line starts with "- " or "* ".
func isBulletItem(line string) bool {
	return strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") || strings.HasPrefix(line, "• ")
}

// stripBulletPrefix removes the bullet marker from the line.
func stripBulletPrefix(line string) string {
	line = strings.TrimPrefix(line, "- ")
	line = strings.TrimPrefix(line, "* ")
	line = strings.TrimPrefix(line, "• ")
	return line
}

// parseNumberedItem extracts the number and text from "1. text" format.
func parseNumberedItem(line string) (int, string, bool) {
	re := regexp.MustCompile(`^(\d+)\.\s+(.+)`)
	m := re.FindStringSubmatch(line)
	if m == nil {
		return 0, "", false
	}
	var num int
	fmt.Sscanf(m[1], "%d", &num)
	return num, m[2], true
}

// renderBulletItem renders a bullet list item as a paragraph with a bullet
// character prefix.
func renderBulletItem(text string) string {
	return fmt.Sprintf(`<w:p><w:pPr><w:ind w:left="720" w:hanging="360"/></w:pPr><w:r><w:t xml:space="preserve">• %s</w:t></w:r></w:p>`,
		html.EscapeString(text))
}

// renderNumberedItem renders a numbered list item.
func renderNumberedItem(num int, text string) string {
	return fmt.Sprintf(`<w:p><w:pPr><w:ind w:left="720" w:hanging="360"/></w:pPr><w:r><w:t xml:space="preserve">%d. %s</w:t></w:r></w:p>`,
		num, html.EscapeString(text))
}

// renderParagraph renders a regular paragraph with inline formatting.
func renderParagraph(text string) string {
	return fmt.Sprintf(`<w:p>%s</w:p>`, renderInlineRuns(text))
}

// renderFigurePlaceholder renders a figure placeholder as an italic note.
func renderFigurePlaceholder(line string) string {
	return fmt.Sprintf(`<w:p><w:pPr><w:jc w:val="center"/></w:pPr><w:r><w:rPr><w:i/></w:rPr><w:t xml:space="preserve">[图表占位: %s]</w:t></w:r></w:p>`,
		html.EscapeString(line))
}

// renderInlineRuns converts inline Markdown (**bold**, *italic*, `code`) to
// OOXML runs within a paragraph. Text outside these markers is escaped.
func renderInlineRuns(text string) string {
	// Pattern matches **bold**, *italic*, or `code`.
	re := regexp.MustCompile(`(\*\*(.+?)\*\*|\*(.+?)\*|\` + "`" + `(.+?)\` + "`" + `)`)

	var buf strings.Builder
	lastEnd := 0
	for _, m := range re.FindAllStringSubmatchIndex(text, -1) {
		// Emit text before the match.
		if m[0] > lastEnd {
			buf.WriteString(renderRun(text[lastEnd:m[0]], false, false, false))
		}
		// Determine which group matched. The regex has an outer capturing
		// group, so inner groups start at index 4, 6, 8.
		if m[4] >= 0 { // **bold** — group 2
			buf.WriteString(renderRun(text[m[4]:m[5]], true, false, false))
		} else if m[6] >= 0 { // *italic* — group 3
			buf.WriteString(renderRun(text[m[6]:m[7]], false, true, false))
		} else if m[8] >= 0 { // `code` — group 4
			buf.WriteString(renderRun(text[m[8]:m[9]], false, false, true))
		}
		lastEnd = m[1]
	}
	// Emit remaining text.
	if lastEnd < len(text) {
		buf.WriteString(renderRun(text[lastEnd:], false, false, false))
	}
	if buf.Len() == 0 {
		buf.WriteString(renderRun(text, false, false, false))
	}
	return buf.String()
}

// renderRun creates a <w:r> element with optional bold/italic/mono styling.
func renderRun(text string, bold, italic, mono bool) string {
	rPr := ""
	if bold || italic || mono {
		rPr = "<w:rPr>"
		if bold {
			rPr += "<w:b/>"
		}
		if italic {
			rPr += "<w:i/>"
		}
		if mono {
			rPr += `<w:rFonts w:ascii="Courier New" w:hAnsi="Courier New"/>`
		}
		rPr += "</w:rPr>"
	}
	return fmt.Sprintf(`<w:r>%s<w:t xml:space="preserve">%s</w:t></w:r>`, rPr, html.EscapeString(text))
}

// parseTableRow splits a "| a | b | c |" line into cells.
func parseTableRow(line string) []string {
	line = strings.TrimPrefix(line, "|")
	line = strings.TrimSuffix(line, "|")
	parts := strings.Split(line, "|")
	for i, p := range parts {
		parts[i] = strings.TrimSpace(p)
	}
	return parts
}

// isTableSeparator returns true for "|---|---|" separator rows.
func isTableSeparator(line string) bool {
	cleaned := strings.ReplaceAll(line, "|", "")
	cleaned = strings.ReplaceAll(cleaned, "-", "")
	cleaned = strings.ReplaceAll(cleaned, " ", "")
	return cleaned == ""
}

// renderTableXML renders a simple OOXML table from rows.
func renderTableXML(rows [][]string) string {
	if len(rows) == 0 {
		return ""
	}
	maxCols := 0
	for _, r := range rows {
		if len(r) > maxCols {
			maxCols = len(r)
		}
	}

	var buf strings.Builder
	// Table properties: full width, with borders.
	buf.WriteString(`<w:tbl><w:tblPr><w:tblW w:w="5000" w:type="pct"/><w:tblBorders>`)
	for _, edge := range []string{"top", "left", "bottom", "right", "insideH", "insideV"} {
		buf.WriteString(fmt.Sprintf(`<w:%s w:val="single" w:sz="4" w:space="0" w:color="auto"/>`, edge))
	}
	buf.WriteString(`</w:tblBorders></w:tblPr>`)

	// Column widths (equal split).
	if maxCols > 0 {
		colW := 9000 / maxCols
		buf.WriteString(`<w:tblGrid>`)
		for i := 0; i < maxCols; i++ {
			buf.WriteString(fmt.Sprintf(`<w:gridCol w:w="%d"/>`, colW))
		}
		buf.WriteString(`</w:tblGrid>`)
	}

	for rowIdx, row := range rows {
		buf.WriteString(`<w:tr>`)
		for i := 0; i < maxCols; i++ {
			cellText := ""
			if i < len(row) {
				cellText = row[i]
			}
			// First row = header (bold).
			isHeader := rowIdx == 0
			shading := ""
			if isHeader {
				shading = `<w:shd w:val="clear" w:color="auto" w:fill="D9E2F3"/>`
			}
			buf.WriteString(fmt.Sprintf(`<w:tc><w:tcPr>%s</w:tcPr><w:p>`, shading))
			if isHeader {
				buf.WriteString(renderRun(cellText, true, false, false))
			} else {
				buf.WriteString(renderRun(cellText, false, false, false))
			}
			buf.WriteString(`</w:p></w:tc>`)
		}
		buf.WriteString(`</w:tr>`)
	}
	buf.WriteString(`</w:tbl>`)
	// Add an empty paragraph after the table (required by OOXML spec).
	buf.WriteString(`<w:p/>`)
	return buf.String()
}
