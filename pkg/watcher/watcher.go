package watcher

import "context"

// EventType describes what kind of change occurred to a record file.
type EventType string

const (
	EventTypeCreated  EventType = "created"
	EventTypeModified EventType = "modified"
	EventTypeDeleted  EventType = "deleted"
)

// FieldChange captures a single field-level change within a record.
type FieldChange struct {
	Field    string
	OldValue any
	NewValue any
}

// RecordEvent represents a change to a single record file.
type RecordEvent struct {
	Type    EventType
	Path    string
	Changes []FieldChange
}

// EventHandler is called when a record changes.
type EventHandler func(RecordEvent)

// Watcher watches a database directory for record changes.
type Watcher interface {
	Watch(ctx context.Context, handler EventHandler) error
}
