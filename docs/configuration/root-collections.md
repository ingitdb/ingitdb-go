# ⚙️ inGitDB Repository Configuration - Root Collections

```yaml
# ⚙️ Each entry maps exactly one collection ID to one collection directory.
rootCollections:
  countries: geo/countries
  todo.tags: todo/tags
  todo.tasks: todo/tasks
```

# 📘 Examples:

- [/.ingitdb.yaml](../../.ingitdb.yaml) - config for inGitDB in this repository.

Collection IDs (the keys in `rootCollections`) must use only alphanumeric characters and `.`,
and must start and end with an alphanumeric character.

Paths in `rootCollections` must point to a single collection directory and cannot use wildcards
such as `*`. This keeps collection IDs explicit and allows GitHub-backed commands to read only
`.ingitdb.yaml` (without extra directory listing API calls), reducing latency.
