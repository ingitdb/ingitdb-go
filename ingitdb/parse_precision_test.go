package ingitdb

import (
	"strings"
	"testing"
)

func TestParseMapOfRecordsContent_PreservesWideInteger(t *testing.T) {
	got, err := ParseMapOfRecordsContent([]byte(`{"row":{"id":9223372036854775807,"small":2}}`), RecordFormatJSON)
	if err != nil {
		t.Fatal(err)
	}
	if got["row"]["id"] != int64(9223372036854775807) {
		t.Fatalf("wide integer lost: %#v", got["row"]["id"])
	}
	if got["row"]["small"] != float64(2) {
		t.Fatalf("small number changed shape: %#v", got["row"]["small"])
	}
}

func TestParseMapOfRecordsContent_RejectsTrailingContent(t *testing.T) {
	_, err := ParseMapOfRecordsContent([]byte(`{"row":{}} {"other":{}}`), RecordFormatJSON)
	if err == nil || !strings.Contains(err.Error(), "trailing") {
		t.Fatalf("wanted trailing content error, got %v", err)
	}
}
