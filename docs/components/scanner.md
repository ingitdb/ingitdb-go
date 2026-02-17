# inGitDB Scanner

Walks over file system objects and invokes components like [validator](validator/schema-validator.md) and [views build](views-builder.md).

```go
package ingitdb

type Scanner interface {
	Scan() (err error) // TODO: define argument and return values 
}

```