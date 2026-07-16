package ingitdb

import (
	"strings"
	"testing"
)

// REQ:foreign-key-enforced — a foreign_key naming a collection absent from the
// definition is rejected at definition-load time. Nothing reads
// ColumnDef.ForeignKey during validation today, so a typo'd target simply
// never resolves and never complains.
func TestValidateForeignKeys_RejectsUnknownTarget(t *testing.T) {
	def := &Definition{Collections: map[string]*CollectionDef{
		"capabilities": {ID: "capabilities", Columns: map[string]*ColumnDef{
			// The realistic failure: a near-miss of the real can-i-use target.
			"equivalenceClass": {Type: ColumnTypeString, ForeignKey: "equivalance_classes"},
		}},
		"equivalence_classes": {ID: "equivalence_classes", Columns: map[string]*ColumnDef{
			"title": {Type: ColumnTypeString},
		}},
	}}
	err := ValidateForeignKeys(def)
	if err == nil {
		t.Fatal("a foreign_key naming an unknown collection must be rejected")
	}
	for _, want := range []string{"equivalenceClass", "equivalance_classes", "capabilities"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error must mention %q, got: %v", want, err)
		}
	}
	// The message lists what IS available, so the typo is obvious.
	if !strings.Contains(err.Error(), "known collections") {
		t.Errorf("error should list the known collections, got: %v", err)
	}
}

// REQ:foreign-key-enforced — module-relative resolution. A bare foreign_key
// resolves within the declaring collection's module first. Mirrors
// demo-ingitdb: commerce.addresses says `foreign_key: countries` and must reach
// commerce.countries, not fail because no bare `countries` exists and not
// collide with geo.countries.
func TestResolveForeignKey_ModuleRelative(t *testing.T) {
	collections := map[string]*CollectionDef{
		"commerce.addresses": {ID: "commerce.addresses"},
		"commerce.countries": {ID: "commerce.countries"},
		"geo.countries":      {ID: "geo.countries"},
	}
	cases := []struct {
		name       string
		declaring  string
		fk         string
		wantTarget string
		wantOK     bool
	}{
		{"bare resolves within module", "commerce.addresses", "countries", "commerce.countries", true},
		{"bare prefers own module over another", "commerce.addresses", "countries", "commerce.countries", true},
		{"fully-qualified is used as-is", "commerce.addresses", "geo.countries", "geo.countries", true},
		{"fully-qualified to a missing target fails", "commerce.addresses", "geo.provinces", "geo.provinces", false},
		{"bare with no module match fails", "commerce.addresses", "nope", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := ResolveForeignKey(tc.declaring, tc.fk, collections)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v (target %q)", ok, tc.wantOK, got)
			}
			if ok && got != tc.wantTarget {
				t.Errorf("target = %q, want %q", got, tc.wantTarget)
			}
		})
	}
}

// A module-relative subcollection FK resolves via the root ancestor's module.
func TestResolveForeignKey_SubcollectionUsesRootModule(t *testing.T) {
	collections := map[string]*CollectionDef{
		"commerce.products": {ID: "commerce.products"},
	}
	got, ok := ResolveForeignKey("commerce.orders/order_details", "products", collections)
	if !ok || got != "commerce.products" {
		t.Errorf("subcollection FK must resolve via root module: got %q ok=%v", got, ok)
	}
}

// A bare collection id (no module) still resolves bare — can-i-use.
func TestResolveForeignKey_NoModuleBare(t *testing.T) {
	collections := map[string]*CollectionDef{
		"capabilities":        {ID: "capabilities"},
		"equivalence_classes": {ID: "equivalence_classes"},
	}
	got, ok := ResolveForeignKey("capabilities", "equivalence_classes", collections)
	if !ok || got != "equivalence_classes" {
		t.Errorf("bare id must resolve bare: got %q ok=%v", got, ok)
	}
}

// ValidateForeignKeys accepts a whole module-namespaced definition. Mirrors
// demo-ingitdb's commerce module.
func TestValidateForeignKeys_AcceptsModuleRelative(t *testing.T) {
	def := &Definition{Collections: map[string]*CollectionDef{
		"commerce.addresses": {ID: "commerce.addresses", Columns: map[string]*ColumnDef{
			"country_id": {Type: ColumnTypeString, ForeignKey: "countries"},
		}},
		"commerce.countries": {ID: "commerce.countries", Columns: map[string]*ColumnDef{
			"name": {Type: ColumnTypeString},
		}},
		"geo.countries": {ID: "geo.countries", Columns: map[string]*ColumnDef{
			"name": {Type: ColumnTypeString},
		}},
	}}
	if err := ValidateForeignKeys(def); err != nil {
		t.Errorf("module-relative foreign_key must resolve, got: %v", err)
	}
}

// A foreign_key naming a real collection loads cleanly. Mirrors can-i-use.
func TestValidateForeignKeys_AcceptsKnownTarget(t *testing.T) {
	def := &Definition{Collections: map[string]*CollectionDef{
		"capabilities": {ID: "capabilities", Columns: map[string]*ColumnDef{
			"equivalenceClass": {Type: ColumnTypeString, ForeignKey: "equivalence_classes"},
		}},
		"equivalence_classes": {ID: "equivalence_classes", Columns: map[string]*ColumnDef{
			"title": {Type: ColumnTypeString},
		}},
	}}
	if err := ValidateForeignKeys(def); err != nil {
		t.Errorf("a foreign_key naming a real collection must load cleanly, got: %v", err)
	}
}

// Subcollections are walked too — a broken FK there is just as invisible.
func TestValidateForeignKeys_WalksSubCollections(t *testing.T) {
	def := &Definition{Collections: map[string]*CollectionDef{
		"orders": {ID: "orders",
			Columns: map[string]*ColumnDef{"id": {Type: ColumnTypeString}},
			SubCollections: map[string]*CollectionDef{
				"order_details": {ID: "order_details", Columns: map[string]*ColumnDef{
					"product": {Type: ColumnTypeString, ForeignKey: "nosuchcollection"},
				}},
			},
		},
	}}
	err := ValidateForeignKeys(def)
	if err == nil {
		t.Fatal("a broken foreign_key in a subcollection must be rejected")
	}
	if !strings.Contains(err.Error(), "orders/order_details") {
		t.Errorf("error must name the subcollection path, got: %v", err)
	}
}

// Every broken reference is reported in one pass, not just the first.
func TestValidateForeignKeys_ReportsAllProblems(t *testing.T) {
	def := &Definition{Collections: map[string]*CollectionDef{
		"a": {ID: "a", Columns: map[string]*ColumnDef{
			"x": {Type: ColumnTypeString, ForeignKey: "missing1"},
			"y": {Type: ColumnTypeString, ForeignKey: "missing2"},
		}},
	}}
	err := ValidateForeignKeys(def)
	if err == nil {
		t.Fatal("expected errors")
	}
	for _, want := range []string{"missing1", "missing2"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("one pass must report %q too, got: %v", want, err)
		}
	}
}

// No foreign keys, and a nil definition, are both fine.
func TestValidateForeignKeys_NoFKsIsClean(t *testing.T) {
	if err := ValidateForeignKeys(nil); err != nil {
		t.Errorf("nil definition must be clean, got: %v", err)
	}
	def := &Definition{Collections: map[string]*CollectionDef{
		"a": {ID: "a", Columns: map[string]*ColumnDef{"x": {Type: ColumnTypeString}}},
	}}
	if err := ValidateForeignKeys(def); err != nil {
		t.Errorf("a definition with no foreign keys must be clean, got: %v", err)
	}
}
