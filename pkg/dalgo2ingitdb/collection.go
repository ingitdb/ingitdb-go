package dalgo2ingitdb

import (
	"fmt"
	"strings"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// CollectionForKey finds the collection and record key for a given ID string.
//
// The id format is "{collectionID}/{recordKey}" where collection IDs use "." for namespaces.
// "/" is reserved for separating collection ID from record key path segments.
// The longest matching collection prefix wins.
func CollectionForKey(def *ingitdb.Definition, id string) (*ingitdb.CollectionDef, string, error) {
	var bestColDef *ingitdb.CollectionDef
	var bestKey string
	var bestLen int

	for colID, colDef := range def.Collections {
		prefix := colID + "/"
		if len(prefix) <= bestLen+1 {
			continue
		}
		if !strings.HasPrefix(id, prefix) {
			continue
		}
		bestLen = len(prefix) - 1
		bestColDef = colDef
		bestKey = id[len(prefix):]
	}

	if bestColDef == nil {
		return nil, "", fmt.Errorf("collection not found for ID %q", id)
	}
	if bestKey == "" {
		return nil, "", fmt.Errorf("no record key in ID %q", id)
	}
	return bestColDef, bestKey, nil
}
