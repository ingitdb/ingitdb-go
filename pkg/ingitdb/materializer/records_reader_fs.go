package materializer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ingitdb/ingitdb-cli/pkg/dalgo2ingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// FileRecordsReader loads records from collection files on disk.
type FileRecordsReader struct {
	readFile func(string) ([]byte, error)
	statFile func(string) (os.FileInfo, error)
	glob     func(string) ([]string, error)
}

func NewFileRecordsReader() FileRecordsReader {
	return FileRecordsReader{
		readFile: os.ReadFile,
		statFile: os.Stat,
		glob:     filepath.Glob,
	}
}

func (r FileRecordsReader) ReadRecords(
	ctx context.Context,
	dbPath string,
	col *ingitdb.CollectionDef,
	yield func(ingitdb.RecordEntry) error,
) error {
	_ = ctx
	_ = dbPath
	if col.RecordFile == nil {
		return fmt.Errorf("collection %q has no record file definition", col.ID)
	}
	fileName := col.RecordFile.Name
	path := filepath.Join(col.DirPath, fileName)
	switch col.RecordFile.RecordType {
	case ingitdb.MapOfIDRecords:
		if _, err := r.statFile(path); err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return fmt.Errorf("failed to stat %s: %w", path, err)
		}
		content, err := r.readFile(path)
		if err != nil {
			return fmt.Errorf("failed to read records file %s: %w", path, err)
		}
		records, err := dalgo2ingitdb.ParseMapOfIDRecordsContent(content, col.RecordFile.Format)
		if err != nil {
			return fmt.Errorf("failed to parse records file %s: %w", path, err)
		}
		for key, data := range records {
			entry := ingitdb.RecordEntry{
				Key:      key,
				FilePath: path,
				Data:     dalgo2ingitdb.ApplyLocaleToRead(data, col.Columns),
			}
			if err := yield(entry); err != nil {
				return err
			}
		}
		return nil
	case ingitdb.SingleRecord:
		patternPath, prefix, suffix, err := recordPatternForKey(fileName, col.DirPath)
		if err != nil {
			return err
		}
		matches, err := r.glob(patternPath)
		if err != nil {
			return fmt.Errorf("failed to glob records: %w", err)
		}
		for _, filePath := range matches {
			content, err := r.readFile(filePath)
			if err != nil {
				return fmt.Errorf("failed to read record %s: %w", filePath, err)
			}
			data, err := dalgo2ingitdb.ParseRecordContent(content, col.RecordFile.Format)
			if err != nil {
				return fmt.Errorf("failed to parse record %s: %w", filePath, err)
			}
			base := filepath.Base(filePath)
			key := strings.TrimSuffix(strings.TrimPrefix(base, prefix), suffix)
			entry := ingitdb.RecordEntry{
				Key:      key,
				FilePath: filePath,
				Data:     dalgo2ingitdb.ApplyLocaleToRead(data, col.Columns),
			}
			if err := yield(entry); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("record type %q is not supported", col.RecordFile.RecordType)
	}
}

func recordPatternForKey(name, dirPath string) (string, string, string, error) {
	const placeholder = "{key}"
	idx := strings.Index(name, placeholder)
	if idx < 0 {
		return "", "", "", fmt.Errorf("record file name %q must include {key}", name)
	}
	prefix := name[:idx]
	suffix := name[idx+len(placeholder):]
	pattern := filepath.Join(dirPath, prefix+"*"+suffix)
	return pattern, prefix, suffix, nil
}
