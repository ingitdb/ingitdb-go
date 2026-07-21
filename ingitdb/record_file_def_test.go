package ingitdb

import (
	dalrecord "github.com/dal-go/record"
	"strings"
	"testing"
)

func TestRecordFileDefValidate_MissingBranches(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		def     RecordFileDef
		wantErr string
	}{
		{
			name: "markdown_requires_single_record",
			def: RecordFileDef{
				Name:       "{key}.md",
				Format:     RecordFormatMarkdown,
				RecordType: ListOfRecords,
			},
			wantErr: "format \"markdown\" requires record type",
		},
		{
			name: "content_field_only_for_markdown",
			def: RecordFileDef{
				Name:         "{key}.yaml",
				Format:       RecordFormatYAML,
				RecordType:   SingleRecord,
				ContentField: "body",
			},
			wantErr: "content_field is only valid for format",
		},
		{
			name: "ingr_rejects_single_record",
			def: RecordFileDef{
				Name:       "records.ingr",
				Format:     RecordFormatINGR,
				RecordType: SingleRecord,
			},
			wantErr: "format \"ingr\" does not support record type",
		},
		{
			name: "invalid_exclude_regex",
			def: RecordFileDef{
				Name:         "{key}.yaml",
				Format:       RecordFormatYAML,
				RecordType:   SingleRecord,
				ExcludeRegex: "[invalid",
			},
			wantErr: "invalid exclude_regex",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.def.Validate()
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}

func TestRecordFileDef_IsExcluded(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		excludeRegex string
		filename     string
		want         bool
	}{
		{
			name:         "empty_regex_never_excludes",
			excludeRegex: "",
			filename:     "README.md",
			want:         false,
		},
		{
			name:         "matching_regex_excludes",
			excludeRegex: `^README\.md$`,
			filename:     "README.md",
			want:         true,
		},
		{
			name:         "non_matching_regex_does_not_exclude",
			excludeRegex: `^README\.md$`,
			filename:     "record-1.md",
			want:         false,
		},
		{
			name:         "invalid_regex_returns_false",
			excludeRegex: "[invalid",
			filename:     "README.md",
			want:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rfd := RecordFileDef{ExcludeRegex: tt.excludeRegex}
			got := rfd.IsExcluded(tt.filename)
			if got != tt.want {
				t.Errorf("IsExcluded(%q) = %v, want %v", tt.filename, got, tt.want)
			}
		})
	}
}

func TestRecordFileDef_ResolvedContentField(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		contentField string
		want         string
	}{
		{
			name:         "empty_returns_default",
			contentField: "",
			want:         DefaultMarkdownContentField,
		},
		{
			name:         "custom_field_returned_as_is",
			contentField: "body",
			want:         "body",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rfd := RecordFileDef{ContentField: tt.contentField}
			got := rfd.ResolvedContentField()
			if got != tt.want {
				t.Errorf("ResolvedContentField() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRecordFileDef_RecordsBasePath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		rfd  RecordFileDef
		want string
	}{
		{
			name: "name_with_key_placeholder_returns_records",
			rfd:  RecordFileDef{Name: "{key}.yaml"},
			want: "$records",
		},
		{
			name: "name_without_key_placeholder_returns_empty",
			rfd:  RecordFileDef{Name: "records.yaml"},
			want: "",
		},
		{
			name: "static_name_returns_empty",
			rfd:  RecordFileDef{Name: "data.json"},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := tt.rfd.RecordsBasePath()
			if got != tt.want {
				t.Errorf("RecordsBasePath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRecordFileDefValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		def  RecordFileDef
		err  string
	}{
		{
			name: "missing_format",
			def:  RecordFileDef{Name: "file.json", RecordType: "map[string]any"},
			err:  "record file format cannot be empty",
		},
		{
			name: "missing_name",
			def:  RecordFileDef{Format: "JSON", RecordType: "map[string]any"},
			err:  "record file name cannot be empty",
		},
		{
			name: "missing_record_type",
			def:  RecordFileDef{Name: "file.json", Format: "JSON"},
			err:  "invalid record type",
		},
		{
			name: "valid",
			def: RecordFileDef{
				Name:       "file.json",
				Format:     "JSON",
				RecordType: "map[string]any",
			},
			err: "",
		},
		{
			name: "valid_map_of_id_records",
			def: RecordFileDef{
				Name:       "records.json",
				Format:     "json",
				RecordType: "map[$record_id]map[$field_name]any",
			},
			err: "",
		},
		{
			name: "csv_rejects_single_record",
			def:  RecordFileDef{Name: "records.csv", Format: RecordFormatCSV, RecordType: SingleRecord},
			err:  "format \"csv\" requires record type \"[]map[string]any\"",
		},
		{
			name: "csv_rejects_map_of_records",
			def:  RecordFileDef{Name: "records.csv", Format: RecordFormatCSV, RecordType: MapOfRecords},
			err:  "format \"csv\" requires record type \"[]map[string]any\"",
		},
		{
			name: "csv_accepts_list_of_records",
			def:  RecordFileDef{Name: "records.csv", Format: RecordFormatCSV, RecordType: ListOfRecords},
			err:  "",
		},
		{
			name: "jsonl_rejects_single_record",
			def:  RecordFileDef{Name: "records.jsonl", Format: RecordFormatJSONL, RecordType: SingleRecord},
			err:  "format \"jsonl\" requires record type \"[]map[string]any\"",
		},
		{
			name: "jsonl_rejects_map_of_records",
			def:  RecordFileDef{Name: "records.jsonl", Format: RecordFormatJSONL, RecordType: MapOfRecords},
			err:  "format \"jsonl\" requires record type \"[]map[string]any\"",
		},
		{
			name: "jsonl_accepts_list_of_records",
			def:  RecordFileDef{Name: "records.jsonl", Format: RecordFormatJSONL, RecordType: ListOfRecords},
			err:  "",
		},
		{
			name: "yaml_accepts_list_of_records",
			def:  RecordFileDef{Name: "records.yaml", Format: RecordFormatYAML, RecordType: ListOfRecords},
			err:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.def.Validate()
			if tt.err == "" && err != nil {
				errMsg := err.Error()
				t.Fatalf("expected no error, got %s", errMsg)
			}
			if tt.err != "" {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				errMsg := err.Error()
				if !strings.Contains(errMsg, tt.err) {
					t.Fatalf("expected error to contain %q, got %q", tt.err, errMsg)
				}
			}
		})
	}
}

func recordWith(t *testing.T, id string, data map[string]any) dalrecord.Record {
	t.Helper()
	key := dalrecord.NewKeyWithID("tasks", id)
	record := dalrecord.NewRecordWithData(key, data)
	record.SetError(nil)
	return record
}

func TestRecordFileDefGetRecordFileName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		def    RecordFileDef
		record dalrecord.Record
		want   string
	}{
		{
			// AC: key-placeholder-still-substitutes
			name:   "key_placeholder_substitutes_record_id",
			def:    RecordFileDef{Format: "JSON", Name: "{key}.yaml"},
			record: recordWith(t, "task-1", map[string]any{"foo": "bar"}),
			want:   "task-1.yaml",
		},
		{
			// AC: fieldname-placeholder-is-substituted
			name:   "fieldname_placeholder_substitutes_column_value",
			def:    RecordFileDef{Format: "JSON", Name: "{status}-{key}.json"},
			record: recordWith(t, "inline-keyboard", map[string]any{"status": "native"}),
			want:   "native-inline-keyboard.json",
		},
		{
			// AC: empty-named-column-is-skipped — an empty-named column is not a
			// placeholder; {key} still substitutes, {} is left untouched.
			name:   "empty_named_column_is_skipped",
			def:    RecordFileDef{Format: "JSON", Name: "{key}.json"},
			record: recordWith(t, "k1", map[string]any{"": "val"}),
			want:   "k1.json",
		},
		{
			name:   "replaces_only_first_key_placeholder",
			def:    RecordFileDef{Format: "JSON", Name: "{key}-second-{key}.json"},
			record: recordWith(t, "task-1", nil),
			want:   "task-1-second-{key}.json",
		},
		{
			// AC: static-name-round-trips
			name:   "static_name",
			def:    RecordFileDef{Format: "JSON", Name: "static.json"},
			record: recordWith(t, "task-1", map[string]any{"foo": "bar"}),
			want:   "static.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := tt.def.GetRecordFileName(tt.record)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestRecordFileDefGetRecordFileName_RejectsPathSeparator(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		def         RecordFileDef
		record      dalrecord.Record
		wantInError []string
	}{
		{
			// AC: slash-in-key-is-rejected
			name:        "slash_in_key",
			def:         RecordFileDef{Format: "JSON", Name: "{key}.json"},
			record:      recordWith(t, "telegram/inline-keyboard", nil),
			wantInError: []string{"key", "telegram/inline-keyboard"},
		},
		{
			// AC: slash-in-field-value-is-rejected
			name:        "slash_in_field_value",
			def:         RecordFileDef{Format: "JSON", Name: "{status}-{key}.json"},
			record:      recordWith(t, "k", map[string]any{"status": "in/progress"}),
			wantInError: []string{"status", "in/progress"},
		},
		{
			name: "backslash_in_key",
			def:  RecordFileDef{Format: "JSON", Name: "{key}.json"},
			// %q escapes the backslash in the message, so assert the placeholder
			// and the error nature rather than the raw value verbatim.
			record:      recordWith(t, `win\path`, nil),
			wantInError: []string{"key", "path separator"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := tt.def.GetRecordFileName(tt.record)
			if err == nil {
				t.Fatalf("expected an error, got file name %q", got)
			}
			if got != "" {
				t.Fatalf("expected empty file name on error, got %q", got)
			}
			for _, want := range tt.wantInError {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("error %q does not contain %q", err.Error(), want)
				}
			}
		})
	}
}
