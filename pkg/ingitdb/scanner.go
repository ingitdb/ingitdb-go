package ingitdb

import "context"

// Scanner orchestrates the full pipeline: walk filesystem, invoke Validator and ViewBuilder.
type Scanner interface {
	// Scan walks dbPath, validates all records, and rebuilds all views.
	Scan(ctx context.Context, dbPath string, def *Definition) error
}
