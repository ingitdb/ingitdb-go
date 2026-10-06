package ingitdb

import (
	"encoding/json"
	"reflect"
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

func TestNormalizeJSONNumberBranches(t *testing.T) {
	cases := []struct {
		in   any
		want any
	}{
		{json.Number("9223372036854775807"), int64(9223372036854775807)},
		{json.Number("1.25"), float64(1.25)},
		{json.Number("bad"), json.Number("bad")},
		{map[string]any{"a": json.Number("2")}, map[string]any{"a": float64(2)}},
		{[]any{json.Number("2")}, []any{float64(2)}},
	}
	for _, tc := range cases {
		if got := normalizeJSONNumber(tc.in); !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("%#v => %#v, want %#v", tc.in, got, tc.want)
		}
	}
	_, err := ParseRecordContent([]byte(`{"a":1} false`), RecordFormatJSON)
	if err == nil || !strings.Contains(err.Error(), "trailing") {
		t.Fatalf("trailing value: %v", err)
	}
	_, err = ParseRecordContent([]byte(`{"a":1} garbage`), RecordFormatJSON)
	if err == nil || !strings.Contains(err.Error(), "trailing") {
		t.Fatalf("trailing text: %v", err)
	}
}
