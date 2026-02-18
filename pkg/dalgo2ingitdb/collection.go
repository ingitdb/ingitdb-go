package dalgo2ingitdb

import (
	"fmt"
	"strings"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// CollectionForKey finds the collection and record key for a given ID string.
//
// The id format is "{collectionID}/{recordKey}" where the collection part may use either
// "." or "/" as a namespace separator. Collection IDs in the definition use "." as separator
// (e.g. "todo.tags"), so both "todo.tags/abc" and "todo/tags/abc" are accepted.
// The longest matching collection prefix wins.
func CollectionForKey(def *ingitdb.Definition, id string) (*ingitdb.CollectionDef, string, error) {
	var bestColDef *ingitdb.CollectionDef
	var bestKey string
	var bestLen int

	for colID, colDef := range def.Collections {
		// Try two prefixes: dot-separated (todo.tags/) and slash-normalized (todo/tags/).
		normalizedColID := strings.ReplaceAll(colID, ".", "/")
		for _, prefix := range []string{colID + "/", normalizedColID + "/"} {
			if !strings.HasPrefix(id, prefix) {
				continue
			}
			if len(prefix) <= bestLen+1 {
				continue
			}
			bestLen = len(prefix) - 1
			bestColDef = colDef
			bestKey = id[len(prefix):]
		}
	}

	if bestColDef == nil {
		return nil, "", fmt.Errorf("collection not found for ID %q", id)
	}
	if bestKey == "" {
		return nil, "", fmt.Errorf("no record key in ID %q", id)
	}
	return bestColDef, bestKey, nil
}
