package ingitdb

// CollectionsReader loads the full database definition from disk.
// The default implementation wraps validator.ReadDefinition.
type CollectionsReader interface {
	ReadDefinition(dbPath string, opts ...ReadOption) (*Definition, error)
}
