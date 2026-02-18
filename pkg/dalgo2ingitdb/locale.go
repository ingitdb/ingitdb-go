package dalgo2ingitdb

import (
	"maps"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// ApplyLocaleToRead transforms record data from file representation to application representation.
// For each column that has a Locale value set (e.g. column "title" with locale "en"),
// the paired map column (e.g. "titles") is inspected: the locale entry is extracted and
// exposed as the shortcut column ("title"), and that locale key is removed from the pair map
// to avoid duplication. The caller receives e.g. {"title": "Work", "titles": {"ru": "Работа"}}.
func ApplyLocaleToRead(data map[string]any, cols map[string]*ingitdb.ColumnDef) map[string]any {
	if len(cols) == 0 {
		return data
	}
	result := maps.Clone(data)
	for colName, colDef := range cols {
		if colDef.Locale == "" {
			continue
		}
		pairField := colName + "s"
		pairVal, ok := result[pairField]
		if !ok {
			continue
		}
		pairMap, ok := pairVal.(map[string]any)
		if !ok {
			continue
		}
		localeVal, exists := pairMap[colDef.Locale]
		if !exists {
			continue
		}
		result[colName] = localeVal
		newPairMap := maps.Clone(pairMap)
		delete(newPairMap, colDef.Locale)
		result[pairField] = newPairMap
	}
	return result
}

// ApplyLocaleToWrite normalises record data before writing to file.
// For each column that has a Locale value set (e.g. column "title" with locale "en"):
//   - The shortcut column ("title") is stored as-is in the file.
//   - If the paired map column ("titles") contains an entry for the primary locale key ("en"),
//     that entry is promoted to the shortcut column and removed from the map, so the value is
//     never duplicated across both fields.
//   - If the paired map becomes empty after removing the primary locale entry, it is dropped
//     from the result to avoid writing a redundant empty map.
func ApplyLocaleToWrite(data map[string]any, cols map[string]*ingitdb.ColumnDef) map[string]any {
	if len(cols) == 0 {
		return data
	}
	result := maps.Clone(data)
	for colName, colDef := range cols {
		if colDef.Locale == "" {
			continue
		}
		pairField := colName + "s"
		pairVal, hasPair := result[pairField]
		if !hasPair {
			continue
		}
		pairMap, ok := pairVal.(map[string]any)
		if !ok {
			continue
		}
		// If the primary locale entry is in the pair map, promote it to the shortcut column.
		if localeVal, exists := pairMap[colDef.Locale]; exists {
			result[colName] = localeVal
			newPairMap := maps.Clone(pairMap)
			delete(newPairMap, colDef.Locale)
			pairMap = newPairMap
		}
		// Drop the pair map if it is now empty.
		if len(pairMap) == 0 {
			delete(result, pairField)
		} else {
			result[pairField] = pairMap
		}
	}
	return result
}
