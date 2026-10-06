package datavalidator

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ingitdb/ingitdb-go/ingitdb"
)

func TestImportedFormatsValidateTransportIDsAndWideValues(t *testing.T) {
	for _, format := range []ingitdb.RecordFormat{ingitdb.RecordFormatJSONL, ingitdb.RecordFormatCSV, ingitdb.RecordFormatINGR} {
		t.Run(string(format), func(t *testing.T) {
			root := t.TempDir()
			colDir := filepath.Join(root, "items")
			if err := os.Mkdir(colDir, 0o755); err != nil {
				t.Fatal(err)
			}
			recordFile := &ingitdb.RecordFileDef{Name: "records." + string(format), Format: format}
			if format == ingitdb.RecordFormatINGR {
				recordFile.RecordType = ingitdb.MapOfRecords
			} else {
				recordFile.RecordType = ingitdb.ListOfRecords
			}
			if format == ingitdb.RecordFormatCSV {
				recordFile.CSVCellEncoding = "json-v1"
			}
			col := &ingitdb.CollectionDef{ID: "items", DirPath: colDir, RecordFile: recordFile,
				Columns: map[string]*ingitdb.ColumnDef{
					"part": {Type: ingitdb.ColumnTypeString}, "number": {Type: ingitdb.ColumnTypeInt},
					"amount": {Type: ingitdb.ColumnTypeString}, "note": {Type: ingitdb.ColumnTypeString},
				},
				ColumnsOrder: []string{"part", "number", "amount", "note"},
				PrimaryKey:   []string{"part", "number"},
				SourceSchema: &ingitdb.SourceSchemaDef{KeyMode: "source-primary-key"},
			}
			row := map[string]any{"part": "A", "number": int64(9007199254740993), "amount": "1.20", "note": nil}
			var content []byte
			var err error
			if format == ingitdb.RecordFormatINGR {
				content, err = ingitdb.EncodeMapOfRecordsContent(map[string]map[string]any{"pk-transport": row}, format, "items", col.ColumnsOrder)
			} else {
				row["$ID"] = "pk-transport"
				if format == ingitdb.RecordFormatCSV {
					col.ColumnsOrder = append([]string{"$ID"}, col.ColumnsOrder...)
					content, err = ingitdb.EncodeRecordContentForCollection([]map[string]any{row}, col)
				} else {
					content, err = ingitdb.EncodeListOfRecordsContent([]map[string]any{row}, format, append([]string{"$ID"}, col.ColumnsOrder...))
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			if err := col.Validate(); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(colDir, recordFile.Name), content, 0o644); err != nil {
				t.Fatal(err)
			}
			def := &ingitdb.Definition{Collections: map[string]*ingitdb.CollectionDef{"items": col}}
			result, err := NewValidator().Validate(context.Background(), root, def)
			if err != nil {
				t.Fatal(err)
			}
			if passed, total := result.GetRecordCounts("items"); passed != 1 || total != 1 || len(result.Errors()) != 0 {
				t.Fatalf("native %s validation: passed=%d total=%d errors=%v", format, passed, total, result.Errors())
			}
		})
	}
}
