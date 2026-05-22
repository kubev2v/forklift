package describe

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"gopkg.in/yaml.v3"

	"github.com/yaacov/kubectl-mtv/pkg/util/output"
)

// Print renders the description in the requested format and writes it to stdout.
func Print(desc *Description, format string) error {
	s, err := Format(desc, format)
	if err != nil {
		return err
	}
	_, err = fmt.Fprint(os.Stdout, s)
	return err
}

// Format renders the description and returns the result as a string.
func Format(desc *Description, format string) (string, error) {
	switch strings.ToLower(format) {
	case "json":
		return formatJSON(desc)
	case "yaml":
		return formatYAML(desc)
	case "markdown":
		return formatMarkdown(desc), nil
	default:
		return formatTable(desc), nil
	}
}

// ---------------------------------------------------------------------------
// JSON
// ---------------------------------------------------------------------------

func formatJSON(desc *Description) (string, error) {
	data, err := json.MarshalIndent(desc, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal description as JSON: %w", err)
	}
	return string(data) + "\n", nil
}

// ---------------------------------------------------------------------------
// YAML
// ---------------------------------------------------------------------------

func formatYAML(desc *Description) (string, error) {
	data, err := yaml.Marshal(desc)
	if err != nil {
		return "", fmt.Errorf("failed to marshal description as YAML: %w", err)
	}
	return string(data), nil
}

// ---------------------------------------------------------------------------
// Markdown
// ---------------------------------------------------------------------------

func formatMarkdown(desc *Description) string {
	var sb strings.Builder
	sb.WriteString("# ")
	sb.WriteString(desc.Title)
	sb.WriteString("\n\n")

	for _, sec := range desc.Sections {
		writeMarkdownSection(&sb, sec, 2)
	}
	return sb.String()
}

func writeMarkdownSection(sb *strings.Builder, sec Section, level int) {
	if sec.Title != "" {
		sb.WriteString(strings.Repeat("#", level))
		sb.WriteString(" ")
		sb.WriteString(sec.Title)
		sb.WriteString("\n\n")
	}

	for _, f := range sec.Fields {
		if f.Label != "" {
			sb.WriteString("- **")
			sb.WriteString(f.Label)
			sb.WriteString(":** ")
		} else {
			sb.WriteString("- ")
		}
		sb.WriteString(f.Value)
		sb.WriteString("\n")
	}
	if len(sec.Fields) > 0 {
		sb.WriteString("\n")
	}

	for _, t := range sec.Tables {
		writeMarkdownTable(sb, t)
	}

	for _, txt := range sec.Texts {
		if txt.Label != "" {
			sb.WriteString("**")
			sb.WriteString(txt.Label)
			sb.WriteString("**\n\n")
		}
		sb.WriteString("```")
		sb.WriteString(txt.Language)
		sb.WriteString("\n")
		sb.WriteString(txt.Content)
		if !strings.HasSuffix(txt.Content, "\n") {
			sb.WriteString("\n")
		}
		sb.WriteString("```\n\n")
	}

	for _, sub := range sec.SubSections {
		writeMarkdownSection(sb, sub, level+1)
	}
}

func writeMarkdownTable(sb *strings.Builder, t Table) {
	if len(t.Headers) == 0 {
		return
	}

	// header row
	sb.WriteString("| ")
	for i, h := range t.Headers {
		if i > 0 {
			sb.WriteString(" | ")
		}
		sb.WriteString(mdEscape(h.Display))
	}
	sb.WriteString(" |\n")

	// separator
	sb.WriteString("|")
	for range t.Headers {
		sb.WriteString(" --- |")
	}
	sb.WriteString("\n")

	// data rows
	for _, row := range t.Rows {
		sb.WriteString("| ")
		for i, h := range t.Headers {
			if i > 0 {
				sb.WriteString(" | ")
			}
			sb.WriteString(mdEscape(row[h.Key]))
		}
		sb.WriteString(" |\n")
	}
	sb.WriteString("\n")
}

func mdEscape(s string) string {
	s = strings.ReplaceAll(s, "|", `\|`)
	s = strings.ReplaceAll(s, "\n", " ")
	return s
}

// ---------------------------------------------------------------------------
// Table (colorized terminal)
// ---------------------------------------------------------------------------

func formatTable(desc *Description) string {
	var sb strings.Builder

	sb.WriteString("\n")
	sb.WriteString(output.Separator(80, output.Yellow))
	sb.WriteString("\n")
	sb.WriteString(output.Bold(output.Cyan(desc.Title)))
	sb.WriteString("\n")

	for _, sec := range desc.Sections {
		writeTableSection(&sb, sec, 0)
	}

	sb.WriteString("\n")
	return sb.String()
}

func writeTableSection(sb *strings.Builder, sec Section, indent int) {
	prefix := strings.Repeat("  ", indent)

	if sec.Title != "" {
		sb.WriteString("\n")
		sb.WriteString(prefix)
		sb.WriteString(output.Bold(output.Cyan(sec.Title)))
		sb.WriteString("\n")
	}

	for _, f := range sec.Fields {
		sb.WriteString(prefix)
		if f.Label != "" {
			sb.WriteString(output.Bold(f.Label+":") + " ")
		}
		if f.ColorFunc != nil {
			sb.WriteString(f.ColorFunc(f.Value))
		} else {
			sb.WriteString(output.Yellow(f.Value))
		}
		sb.WriteString("\n")
	}

	for _, t := range sec.Tables {
		writeTableTable(sb, t, prefix)
	}

	for _, txt := range sec.Texts {
		if txt.Label != "" {
			sb.WriteString("\n")
			sb.WriteString(prefix)
			sb.WriteString(output.Bold(txt.Label + ":"))
			sb.WriteString("\n")
		}
		for _, line := range strings.Split(txt.Content, "\n") {
			sb.WriteString(prefix)
			sb.WriteString("  ")
			sb.WriteString(line)
			sb.WriteString("\n")
		}
	}

	for _, sub := range sec.SubSections {
		writeTableSection(sb, sub, indent+1)
	}
}

func writeTableTable(sb *strings.Builder, t Table, prefix string) {
	if len(t.Headers) == 0 || len(t.Rows) == 0 {
		return
	}

	headers := make([]string, len(t.Headers))
	for i, h := range t.Headers {
		headers[i] = h.Display
	}

	rows := make([][]string, 0, len(t.Rows))
	for _, row := range t.Rows {
		r := make([]string, len(t.Headers))
		for j, h := range t.Headers {
			val := row[h.Key]
			if h.ColorFunc != nil {
				val = h.ColorFunc(val)
			}
			r[j] = val
		}
		rows = append(rows, r)
	}

	tbl := table.New().
		Headers(headers...).
		Rows(rows...).
		BorderTop(false).
		BorderBottom(false).
		BorderLeft(false).
		BorderRight(false).
		BorderColumn(false).
		BorderHeader(true).
		BorderStyle(lipgloss.NewStyle()).
		StyleFunc(func(row, col int) lipgloss.Style {
			s := lipgloss.NewStyle().PaddingRight(2)
			if row == table.HeaderRow && output.IsColorEnabled() {
				return s.Bold(true)
			}
			return s
		})

	for _, line := range strings.Split(tbl.Render(), "\n") {
		sb.WriteString(prefix)
		sb.WriteString(line)
		sb.WriteString("\n")
	}
}
