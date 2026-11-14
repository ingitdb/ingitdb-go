package validator

// specscore: feature/column-validation

import (
	"sort"
	"strings"
	"testing"

	"github.com/ingitdb/ingitdb-go/ingitdb"
)

// fkPathDB is a Firestore-shaped database: spaces/{space}/ext/{ext}/... and
// ext/{ext}/..., with record files next to their subcollection directories.
func fkPathDB(t *testing.T, extra map[string]string) string {
	t.Helper()
	files := map[string]string{
		"spaces/.collection/definition.yaml":                                              firestoreStyleParent,
		"spaces/.collection/subcollections/ext/definition.yaml":                           firestoreStyleParent,
		"spaces/.collection/subcollections/ext/subcollections/agreements/definition.yaml": firestoreStyleParent,
		"ext/.collection/definition.yaml":                                                 firestoreStyleParent,
		"ext/.collection/subcollections/sessions/definition.yaml":                         firestoreStyleParent,
		"spaces/s1.json": `{"name": "S1"}`,
		"spaces/s2.json": `{"name": "S2"}`,
		"spaces/s1/ext/calendarius/agreements/a1.json": `{"name": "A1"}`,
		"ext/commitius/sessions/sess1.json":            `{"name": "Session 1"}`,
	}
	for path, content := range extra {
		files[path] = content
	}
	roots := map[string]string{"spaces": "./spaces", "ext": "./ext"}
	if _, hasBookings := extra["bookings/.collection/definition.yaml"]; hasBookings {
		roots["bookings"] = "./bookings"
	}
	return writeDB(t, roots, files)
}

func childWithFK(column, foreignKey string, extraColumns string) string {
	return firestoreStyleParent + "  " + column + ":\n    type: any\n    foreign_key: " + foreignKey + "\n" + extraColumns
}

// Verifies column-validation#ac:subcollection-foreign-key-literal-parent.
func TestForeignKeyPath_LiteralParent(t *testing.T) {
	t.Parallel()
	root := fkPathDB(t, map[string]string{
		"spaces/.collection/subcollections/ext/subcollections/checkIns/definition.yaml": childWithFK("sessionID", "ext/commitius/sessions", ""),
		"spaces/s1/ext/commitius/checkIns/c1.json":                                      `{"sessionID": "sess1"}`,
		"spaces/s1/ext/commitius/checkIns/c2.json":                                      `{"sessionID": "ghost"}`,
		"spaces/s1/ext/commitius/checkIns/c3.json":                                      `{"sessionID": ["sess1", "ghost2", ["nested"]]}`,
		"spaces/s1/ext/commitius/checkIns/c4.json":                                      `{"sessionID": null}`,
		"spaces/s1/ext/commitius/checkIns/c5.json":                                      `{"sessionID": ""}`,
	})
	errs := findingsFor(validateResult(t, root).Errors(), "spaces/ext/checkIns")
	var messages []string
	for _, e := range errs {
		messages = append(messages, e.RecordKey+": "+e.Message)
	}
	joined := strings.Join(messages, "\n")
	if len(errs) != 3 {
		t.Fatalf("expected 3 findings (ghost, ghost2, a nested element), got:\n%s", joined)
	}
	for _, want := range []string{
		`c2: foreign key "sessionID" = "ghost" has no matching record in collection "ext/commitius/sessions"`,
		`c3: foreign key "sessionID" = "ghost2" has no matching record`,
		`c3: foreign key "sessionID" has a list element that is not a scalar`,
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing finding %q in:\n%s", want, joined)
		}
	}
}

// Verifies column-validation#ac:subcollection-foreign-key-same-parent.
// A grant may only point at an agreement of its own space and extension.
func TestForeignKeyPath_SameParentKeys(t *testing.T) {
	t.Parallel()
	root := fkPathDB(t, map[string]string{
		"spaces/.collection/subcollections/ext/subcollections/grants/definition.yaml": childWithFK("agreementID", "spaces/$same/ext/$same/agreements", ""),
		"spaces/s1/ext/calendarius/grants/g1.json":                                    `{"agreementID": "a1"}`,
		"spaces/s2/ext/calendarius/grants/g2.json":                                    `{"agreementID": "a1"}`,
	})
	errs := findingsFor(validateResult(t, root).Errors(), "spaces/ext/grants")
	if len(errs) != 1 || errs[0].RecordKey != "g2" || !strings.Contains(errs[0].Message, `"spaces/s2/ext/calendarius/agreements"`) {
		t.Fatalf("expected only g2 (other space) to dangle, got %v", errs)
	}
}

// Verifies column-validation#ac:subcollection-foreign-key-parent-from-field.
func TestForeignKeyPath_ParentKeyFromField(t *testing.T) {
	t.Parallel()
	bookings := childWithFK("agreementID", "spaces/{spaceID}/ext/calendarius/agreements", "  spaceID:\n    type: string\n")
	root := fkPathDB(t, map[string]string{
		"bookings/.collection/definition.yaml": bookings,
		"bookings/b1.json":                     `{"spaceID": "s1", "agreementID": "a1"}`,
		"bookings/b2.json":                     `{"spaceID": "s2", "agreementID": "a1"}`,
		"bookings/b3.json":                     `{"agreementID": "a1"}`,
	})
	errs := validateResult(t, root).Errors()
	got := map[string]string{}
	for _, e := range findingsFor(errs, "bookings") {
		got[e.RecordKey] = e.Message
	}
	if len(got) != 2 {
		t.Fatalf("expected findings for b2 and b3, got %v", got)
	}
	if !strings.Contains(got["b2"], `"spaces/s2/ext/calendarius/agreements"`) {
		t.Errorf("b2 must dangle in space s2, got %q", got["b2"])
	}
	if !strings.Contains(got["b3"], `field "spaceID"`) || !strings.Contains(got["b3"], "is empty") {
		t.Errorf("b3 has no spaceID, got %q", got["b3"])
	}
}

// Verifies column-validation#ac:subcollection-foreign-key-checked-at-load.
func TestForeignKeyPath_InvalidPathsRejectedAtLoad(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, foreignKey, want string
	}{
		{"even segments", "ext/commitius", "must alternate"},
		{"empty segment", "ext//sessions", "empty segment"},
		{"unknown root", "nowhere/x/sessions", `root collection "nowhere"`},
		{"unknown subcollection", "ext/commitius/nope", `"ext" has no subcollection "nope"`},
		{"same off the declaring path", "ext/$same/sessions", "$same after \"ext\""},
		{"undeclared field", "ext/{nope}/sessions", `field "nope" is not a column`},
	}
	for _, tt := range tests {
		root := fkPathDB(t, map[string]string{
			"spaces/.collection/subcollections/ext/subcollections/checkIns/definition.yaml": childWithFK("ref", tt.foreignKey, ""),
		})
		_, err := ReadDefinition(root, ingitdb.Validate())
		if err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%s: expected load error containing %q, got %v", tt.name, tt.want, err)
		}
	}
}

// A root foreign_key resolves exactly as before, and a path is only tried
// when the value does not name a root collection.
func TestForeignKeyPath_RootForeignKeyUnchanged(t *testing.T) {
	t.Parallel()
	root := fkPathDB(t, map[string]string{
		"spaces/.collection/subcollections/ext/subcollections/checkIns/definition.yaml": childWithFK("spaceID", "spaces", ""),
		"spaces/s1/ext/commitius/checkIns/c1.json":                                      `{"spaceID": "s9"}`,
	})
	errs := findingsFor(validateResult(t, root).Errors(), "spaces/ext/checkIns")
	if len(errs) != 1 || !strings.Contains(errs[0].Message, `no matching record in collection "spaces"`) {
		t.Fatalf("expected the unchanged root foreign key finding, got %v", errs)
	}
}

// Verifies column-validation#ac:nested-shape-declared.
// columns / values / items decode from a real definition.yaml and are checked.
func TestColumnShape_E2EFromDefinitionYAML(t *testing.T) {
	t.Parallel()
	happenings := firestoreStyleParent + `  slots:
    type: map[string]any
    values:
      type: map[string]any
      columns:
        durationInMinutes:
          type: int
          required: true
        weekdays:
          type: "[]string"
          items:
            type: string
            enum: [mo, tu, we, th, fr, sa, su]
`
	root := fkPathDB(t, map[string]string{
		"spaces/.collection/subcollections/ext/subcollections/happenings/definition.yaml": happenings,
		"spaces/s1/ext/calendarius/happenings/h1.json":                                    `{"slots": {"sl1": {"durationInMinutes": "x", "weekdays": ["mo", "xx"]}}}`,
	})
	got := findingMessagesE2E(findingsFor(validateResult(t, root).Errors(), "spaces/ext/happenings"))
	want := []string{"slots.sl1.durationInMinutes", "slots.sl1.weekdays[1]"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("findings on fields %v, want %v", got, want)
	}
}

func findingMessagesE2E(errs []ingitdb.ValidationError) []string {
	var fields []string
	for _, e := range errs {
		fields = append(fields, e.FieldName)
	}
	sort.Strings(fields)
	return fields
}
