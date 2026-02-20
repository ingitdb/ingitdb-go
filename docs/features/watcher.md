# 📘 inGitDB Watcher

The watcher monitors an inGitDB directory for file-system changes and emits structured events describing what changed and how.

## 📂 Status

Pending.

## 📂 Use cases

- Live development: see record changes in real time as a teammate (or tool) edits the DB.
- Debugging: trace which records are affected by an operation.
- Automation: pipe events to scripts or other tools.

## 📂 Output format

Events are written to **stdout**, one line per event, so the output can be piped or redirected freely.

### 🔹 Text format (default)

```
Record /countries/gb/cities/london: added
Record /countries/gb/cities/london: 2 fields updated: {population: 9000000, area: 1572}
Record /countries/gb/cities/london: deleted
```

### 🔹 JSON format (`--format=json`)

```json
{"type":"added","record":"/countries/gb/cities/london"}
{"type":"updated","record":"/countries/gb/cities/london","fields":{"population":9000000,"area":1572}}
{"type":"deleted","record":"/countries/gb/cities/london"}
```

## 📂 CLI

```
ingitdb watch [--path=PATH] [--format=text|json]
```

See [CLI reference](../CLI.md#watch--watch-database-for-changes-not-yet-implemented).

## 📂 Related

- [Subscribers](subscribers.md) — configurable event handlers (webhooks, email, shell) triggered by the same events.
- [Triggers](../components/triggers.md) — pluggable scripts called on data change.
