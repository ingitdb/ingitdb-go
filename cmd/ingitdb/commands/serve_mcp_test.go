package commands

import (
	"testing"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

func TestSortedCollectionIDs_ReturnsAllNamespacedIDs(t *testing.T) {
	t.Parallel()

	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"countries":     {ID: "countries"},
			"todo.tags":     {ID: "todo.tags"},
			"todo.tasks":    {ID: "todo.tasks"},
			"todo.statuses": {ID: "todo.statuses"},
		},
	}

	got := sortedCollectionIDs(def)
	want := []string{"countries", "todo.statuses", "todo.tags", "todo.tasks"}

	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i, id := range want {
		if got[i] != id {
			t.Errorf("got[%d] = %q, want %q", i, got[i], id)
		}
	}
}

func TestSortedCollectionIDs_DoesNotCollapseNamespace(t *testing.T) {
	t.Parallel()

	// Before the fix, list_collections collapsed todo.tags/todo.tasks/todo.statuses
	// into a single "todo" root prefix entry. Verify this no longer happens.
	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"todo.tags":  {ID: "todo.tags"},
			"todo.tasks": {ID: "todo.tasks"},
		},
	}

	got := sortedCollectionIDs(def)
	if len(got) != 2 {
		t.Fatalf("expected 2 collection IDs (one per collection), got %v", got)
	}
	if got[0] == "todo" || got[1] == "todo" {
		t.Errorf("namespace root 'todo' must not appear as a collection; got %v", got)
	}
}

func TestSortedCollectionIDs_Empty(t *testing.T) {
	t.Parallel()

	def := &ingitdb.Definition{Collections: map[string]*ingitdb.CollectionDef{}}
	got := sortedCollectionIDs(def)
	if len(got) != 0 {
		t.Errorf("expected empty slice, got %v", got)
	}
}
