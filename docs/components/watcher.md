# 🧩 inGitDB Watcher Component

Watches an inGitDB directory for file-system changes, maps each changed path to a record event, and delivers those events to registered handlers.

## 📂 Responsibilities

1. **Detect changes** — use OS file-system notifications (e.g. `fsnotify`) to receive create/write/delete events on the DB directory tree.
2. **Resolve record identity** — map the changed file path back to a collection key and record ID using the loaded `Definition`.
3. **Classify the change** — determine whether the record was added, updated (and which fields changed), or deleted.
4. **Emit events** — call registered `EventHandler` callbacks with a typed `RecordEvent`.

## 📂 Interfaces

```go
package watcher

type EventType string

const (
    EventAdded   EventType = "added"
    EventUpdated EventType = "updated"
    EventDeleted EventType = "deleted"
)

type FieldChange struct {
    Field    string
    OldValue any
    NewValue any
}

type RecordEvent struct {
    Type    EventType
    Path    string // e.g. /countries/gb/cities/london
    Changes []FieldChange // non-nil only for EventUpdated
}

type EventHandler func(RecordEvent)

type Watcher interface {
    // Watch starts watching and blocks until ctx is cancelled or an error occurs.
    Watch(ctx context.Context, handler EventHandler) error
}
```

## 📂 Text formatting

The `text` formatter renders events in the human-readable form logged to stdout by `ingitdb watch`:

```
Record /countries/gb/cities/london: added
Record /countries/gb/cities/london: 2 fields updated: {population: 9000000, area: 1572}
Record /countries/gb/cities/london: deleted
```

## 📂 Related

- [Feature: Watcher](../features/watcher.md)
- [Triggers](triggers.md)
- [Scanner](scanner.md)
