package ingitdb

// RecordEntry is one parsed record with its location on disk.
type RecordEntry struct {
	Key      string // may be empty for list-type files
	FilePath string // absolute path
	Data     map[string]any
}
