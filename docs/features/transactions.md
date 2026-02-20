# 📘 inGitDB Transactions

inGitDB supports read-only and read-write transactions through the DALgo abstraction layer.

The `pkg/dalgo2ingitdb` package implements the `dal.DB` interface from
[DALgo](https://github.com/dal-go/dalgo), providing:

- **Read-only transactions** (`tx_readonly.go`) — load records from a collection without acquiring a write lock.
- **Read-write transactions** (`tx_readwrite.go`) — load, modify, and persist records atomically.

## 📂 Related

- [DALgo](https://github.com/dal-go/dalgo) — the database abstraction layer inGitDB implements.
- [Package `dalgo2ingitdb`](../../pkg/dalgo2ingitdb/) — Go source for the transaction layer.