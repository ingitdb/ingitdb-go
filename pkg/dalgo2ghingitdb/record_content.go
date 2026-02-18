package dalgo2ghingitdb

import (
	"path"
	"strings"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

func resolveRecordPath(colDef *ingitdb.CollectionDef, recordKey string) string {
	recordName := strings.ReplaceAll(colDef.RecordFile.Name, "{key}", recordKey)
	recordPath := path.Join(colDef.DirPath, recordName)
	return path.Clean(recordPath)
}
