package ingitdb

import "testing"

// ResolveListRecordKey must recognise the canonical reserved identity field
// "$ID" (the form datavalidator injects and the form declared as a column in
// demo-ingitdb's order_details), in addition to the historical lowercase "$id"
// and bare "id". Without "$ID" every list record keyed that way resolves to no
// key. Locks the fix that lets subcollection list records be validated.
func TestResolveListRecordKey_IdentityFieldForms(t *testing.T) {
	cases := []struct {
		name    string
		row     map[string]any
		wantKey string
		wantOK  bool
	}{
		{"canonical $ID", map[string]any{"$ID": "od1", "v": 1}, "od1", true},
		{"lowercase $id", map[string]any{"$id": "od2"}, "od2", true},
		{"bare id", map[string]any{"id": "od3"}, "od3", true},
		{"no identity field", map[string]any{"v": 1}, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			key, ok := ResolveListRecordKey(tc.row, &CollectionDef{})
			if key != tc.wantKey || ok != tc.wantOK {
				t.Errorf("ResolveListRecordKey(%v) = (%q, %v), want (%q, %v)", tc.row, key, ok, tc.wantKey, tc.wantOK)
			}
		})
	}
}

// A declared primary_key still wins over any identity field.
func TestResolveListRecordKey_PrimaryKeyWins(t *testing.T) {
	row := map[string]any{"$ID": "ignored", "country": "US", "region": "CA"}
	col := &CollectionDef{PrimaryKey: []string{"country", "region"}}
	key, ok := ResolveListRecordKey(row, col)
	if !ok || key != "US"+listKeySeparator+"CA" {
		t.Errorf("primary_key must win, got (%q, %v)", key, ok)
	}
}
