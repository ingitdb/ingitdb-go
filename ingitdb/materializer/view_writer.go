package materializer

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	"github.com/ingitdb/ingitdb-go/ingitdb"
)

type FuncViewWriter struct {
	write func(content []byte) error
}

func NewFuncViewWriter(write func(content []byte) error) FuncViewWriter {
	return FuncViewWriter{write: write}
}

func (w FuncViewWriter) WriteView(
	ctx context.Context,
	col *ingitdb.CollectionDef,
	view *ingitdb.ViewDef,
	records []ingitdb.IRecordEntry,
	outPath string,
) (WriteOutcome, error) {
	_ = ctx
	if view.Template == "" {
		content, err := renderBuiltinView(view, records)
		if err != nil {
			return WriteOutcomeUnchanged, err
		}
		if err := w.write(content); err != nil {
			return WriteOutcomeUnchanged, err
		}
		return WriteOutcomeCreated, nil
	}
	templatePath := filepath.Join(col.DirPath, view.Template)
	tmpl, err := template.ParseFiles(templatePath)
	if err != nil {
		return WriteOutcomeUnchanged, fmt.Errorf("failed to parse template %s: %w", templatePath, err)
	}
	data := viewTemplateData(view, records)
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return WriteOutcomeUnchanged, fmt.Errorf("failed to render template %s: %w", templatePath, err)
	}
	content := buf.Bytes()
	if strings.HasSuffix(strings.ToLower(outPath), ".md") {
		content = stripMarkdownComments(content)
	}
	if err := w.write(content); err != nil {
		return WriteOutcomeUnchanged, err
	}
	return WriteOutcomeCreated, nil
}

// FileViewWriter renders a view template and writes it to a file.
type FileViewWriter struct {
	readFile  func(string) ([]byte, error)
	writeFile func(string, []byte, os.FileMode) error
	mkdirAll  func(string, os.FileMode) error
}

func NewFileViewWriter() FileViewWriter {
	return FileViewWriter{
		readFile:  os.ReadFile,
		writeFile: os.WriteFile,
		mkdirAll:  os.MkdirAll,
	}
}

func (w FileViewWriter) WriteView(
	ctx context.Context,
	col *ingitdb.CollectionDef,
	view *ingitdb.ViewDef,
	records []ingitdb.IRecordEntry,
	outPath string,
) (WriteOutcome, error) {
	_ = ctx
	if view.Template == "" {
		content, err := renderBuiltinView(view, records)
		if err != nil {
			return WriteOutcomeUnchanged, err
		}
		existing, readErr := w.readFile(outPath)
		if readErr == nil {
			if bytes.Equal(existing, content) {
				return WriteOutcomeUnchanged, nil
			}
		}
		if err := w.mkdirAll(filepath.Dir(outPath), 0o755); err != nil {
			return WriteOutcomeUnchanged, fmt.Errorf("failed to create directory for %s: %w", outPath, err)
		}
		if err := w.writeFile(outPath, content, 0o644); err != nil {
			return WriteOutcomeUnchanged, fmt.Errorf("failed to write view output %s: %w", outPath, err)
		}
		if readErr == nil {
			return WriteOutcomeUpdated, nil
		}
		return WriteOutcomeCreated, nil
	}
	templatePath := filepath.Join(col.DirPath, view.Template)
	tmpl, err := template.ParseFiles(templatePath)
	if err != nil {
		return WriteOutcomeUnchanged, fmt.Errorf("failed to parse template %s: %w", templatePath, err)
	}
	data := viewTemplateData(view, records)
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return WriteOutcomeUnchanged, fmt.Errorf("failed to render template %s: %w", templatePath, err)
	}
	content := buf.Bytes()
	if strings.HasSuffix(strings.ToLower(outPath), ".md") {
		content = stripMarkdownComments(content)
	}
	existing, readErr := w.readFile(outPath)
	if readErr == nil {
		if bytes.Equal(existing, content) {
			return WriteOutcomeUnchanged, nil
		}
	}
	if err := w.mkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return WriteOutcomeUnchanged, fmt.Errorf("failed to create directory for %s: %w", outPath, err)
	}
	if err := w.writeFile(outPath, content, 0o644); err != nil {
		return WriteOutcomeUnchanged, fmt.Errorf("failed to write view output %s: %w", outPath, err)
	}
	if readErr == nil {
		return WriteOutcomeUpdated, nil
	}
	return WriteOutcomeCreated, nil
}

// renderBuiltinView renders a view using a built-in renderer (no template file).
// It checks view.Formats for "md" and renders a markdown table.
// If no supported format is found, it returns an error.
func renderBuiltinView(view *ingitdb.ViewDef, records []ingitdb.IRecordEntry) ([]byte, error) {
	for _, f := range view.Formats {
		if strings.EqualFold(f, "md") {
			return renderBuiltinMDTable(view, records), nil
		}
	}
	return nil, fmt.Errorf("view template is required")
}

// renderBuiltinMDTable renders records as a markdown pipe table.
// Uses view.Columns as headers; if empty, collects keys from first record sorted alphabetically.
func renderBuiltinMDTable(view *ingitdb.ViewDef, records []ingitdb.IRecordEntry) []byte {
	cols := view.Columns
	if len(cols) == 0 && len(records) > 0 {
		data := records[0].GetData()
		keys := make([]string, 0, len(data))
		for k := range data {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		cols = keys
	}

	var sb strings.Builder

	// Header row
	sb.WriteString("|")
	for _, col := range cols {
		sb.WriteString(" ")
		sb.WriteString(col)
		sb.WriteString(" |")
	}
	sb.WriteString("\n")

	// Separator row
	sb.WriteString("|")
	for range cols {
		sb.WriteString("---|")
	}
	sb.WriteString("\n")

	// Data rows
	for _, record := range records {
		d := record.GetData()
		sb.WriteString("|")
		for _, col := range cols {
			sb.WriteString(" ")
			v := d[col]
			if v != nil {
				fmt.Fprintf(&sb, "%v", v)
			}
			sb.WriteString(" |")
		}
		sb.WriteString("\n")
	}

	return []byte(sb.String())
}

func viewTemplateData(view *ingitdb.ViewDef, records []ingitdb.IRecordEntry) map[string]any {
	varName := view.RecordsVarName
	if varName == "" {
		varName = "records"
	}
	items := make([]map[string]any, 0, len(records))
	for _, record := range records {
		items = append(items, record.GetData())
	}
	return map[string]any{
		varName: items,
	}
}

func stripMarkdownComments(content []byte) []byte {
	text := string(content)
	lines := strings.Split(text, "\n")
	filtered := make([]string, 0, len(lines))
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if isMarkdownCommentLine(line) {
			if len(filtered) > 0 && isEmptyLine(filtered[len(filtered)-1]) {
				if i+1 < len(lines) && isEmptyLine(lines[i+1]) {
					filtered = filtered[:len(filtered)-1]
				}
			}
			continue
		}
		filtered = append(filtered, line)
	}
	result := strings.Join(filtered, "\n")
	if strings.HasSuffix(text, "\n") && !strings.HasSuffix(result, "\n") {
		result += "\n"
	}
	return []byte(result)
}

func isMarkdownCommentLine(line string) bool {
	trimmed := strings.TrimSpace(strings.TrimSuffix(line, "\r"))
	if !strings.HasPrefix(trimmed, "[//]:") {
		return false
	}
	return strings.Contains(trimmed, "#")
}

func isEmptyLine(line string) bool {
	return strings.TrimSpace(strings.TrimSuffix(line, "\r")) == ""
}
