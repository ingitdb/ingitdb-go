// Package todo holds the TODO demo's records: two lists, To buy and To
// watch, each with a few items. OpenVaultDB's ovdb demo install and
// ingitdb-cli's ingitdb demo install both seed the same records from this
// package, so the two demos never drift (ingitdb-cli cli/demo#REQ:shared-demo-data,
// OpenVaultDB todo-demo#REQ:seed-from-shared-package). It imports only the Go
// standard library, so neither consumer takes on inGitDB's engine API or
// OpenVaultDB's.
package todo

// specscore: feature/cli/demo

import "time"

// App is the demo name commands default to.
const App = "todo"

// Lists are the demo's list paths, without a leading slash, in the order
// apps show them.
var Lists = []string{"lists/to-buy", "lists/to-watch"}

// Record is one record the demo writes: Key has no leading slash, Data is
// the record's fields.
type Record struct {
	Key  string
	Data map[string]any
}

// Records returns the TODO demo's records: each list record immediately
// before its items, in the order lists are shown and items were added.
// Every item has done: false and an added_at RFC 3339 UTC timestamp; the
// five items are one second apart, the last (interstellar) at now truncated
// to the second, so no item is in the future and every client lists them in
// the same order.
func Records(now time.Time) []Record {
	at := now.UTC().Truncate(time.Second).Add(-4 * time.Second)
	addedAt := func() string {
		s := at.Format(time.RFC3339)
		at = at.Add(time.Second)
		return s
	}
	return []Record{
		{Key: "lists/to-buy", Data: map[string]any{"title": "To buy"}},
		{Key: "lists/to-buy/items/milk", Data: map[string]any{
			"title": "Milk", "done": false, "added_at": addedAt(),
		}},
		{Key: "lists/to-buy/items/bananas", Data: map[string]any{
			"title": "Bananas", "done": false, "added_at": addedAt(),
		}},
		{Key: "lists/to-buy/items/coffee", Data: map[string]any{
			"title": "Coffee", "done": false, "added_at": addedAt(),
		}},
		{Key: "lists/to-watch", Data: map[string]any{"title": "To watch"}},
		{Key: "lists/to-watch/items/the-matrix", Data: map[string]any{
			"title": "The Matrix", "done": false, "added_at": addedAt(),
		}},
		{Key: "lists/to-watch/items/interstellar", Data: map[string]any{
			"title": "Interstellar", "done": false, "added_at": addedAt(),
		}},
	}
}
