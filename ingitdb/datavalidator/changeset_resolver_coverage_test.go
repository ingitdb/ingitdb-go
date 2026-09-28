package datavalidator

import (
	"testing"

	"github.com/ingitdb/ingitdb-go/ingitdb"
)

func TestCollectionForRecordFile_EdgeCases(t *testing.T) {
	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"skip": {
				RecordFile: nil, // shouldSkipRecordParsing == true
			},
			"bad_pattern": {
				RecordFile: &ingitdb.RecordFileDef{
					RecordType: ingitdb.SingleRecord,
					Format:     "unknown_format",
				},
			},
		},
	}
	id, col := CollectionForRecordFile(def, "/any/path.yaml")
	if id != "" || col != nil {
		t.Errorf("expected empty result, got id=%q, col=%v", id, col)
	}
}
