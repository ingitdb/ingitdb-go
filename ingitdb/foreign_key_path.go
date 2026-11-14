package ingitdb

// specscore: feature/column-validation

import (
	"fmt"
	"strings"
)

// ForeignKeySameKey is the key segment of a foreign_key path that means "the
// key the referring record itself has at this level". Record keys never start
// with "$" ($records, $views and $ID are inGitDB's own names), so it cannot
// collide with a literal key.
const ForeignKeySameKey = "$same"

// ForeignKeyPath is a foreign_key that names a subcollection rather than a root
// collection. It is written like a Firestore path without the final id,
// alternating collection ids and parent keys:
//
//	ext/commitius/cmSessions                  literal parent key
//	spaces/$same/ext/$same/financialAgreements the referring record's own keys
//	spaces/{spaceID}/ext/calendarius/happenings a key from the record's field
//
// Root is the resolved root collection id (module-relative, as
// ResolveForeignKey), and Steps descend from it one subcollection per step.
type ForeignKeyPath struct {
	Root  string
	Steps []ForeignKeyPathStep
}

// ForeignKeyPathStep is one descent: the key of the parent record, then the id
// of its subcollection. Exactly one of Literal, Field and Same describes the
// key.
type ForeignKeyPathStep struct {
	Literal    string // a fixed parent key, e.g. "commitius"
	Field      string // the parent key is the referring record's value of this field
	Same       bool   // the parent key is the referring record's own key at this level
	Collection string // the subcollection id below that parent
}

// IsForeignKeyPath reports whether fk is written as a subcollection path (it
// contains "/"). A root foreign_key never does, so readers that only resolve
// root targets can use this to leave path targets to the validator.
func IsForeignKeyPath(fk string) bool {
	return strings.Contains(fk, "/")
}

// String renders the path back in its declared form, with Root resolved.
func (p ForeignKeyPath) String() string {
	parts := []string{p.Root}
	for _, step := range p.Steps {
		parts = append(parts, step.keySpec(), step.Collection)
	}
	return strings.Join(parts, "/")
}

func (s ForeignKeyPathStep) keySpec() string {
	switch {
	case s.Same:
		return ForeignKeySameKey
	case s.Field != "":
		return "{" + s.Field + "}"
	default:
		return s.Literal
	}
}

// ResolveForeignKeyPath parses and checks a path foreign_key declared by column
// columnName of the collection at declaringFullID (its full schema path, e.g.
// "spaces/ext/cmCheckIns"). The root segment resolves module-relative, every
// collection segment must be a declared subcollection, a `{field}` key must
// name a column of the declaring collection, and `$same` is only allowed while
// the path follows the declaring collection's own schema path (level by level,
// including the declaring collection itself).
func ResolveForeignKeyPath(declaringFullID, fk string, declaring *CollectionDef, collections map[string]*CollectionDef) (*ForeignKeyPath, error) {
	segments := strings.Split(fk, "/")
	if len(segments)%2 == 0 {
		return nil, fmt.Errorf("foreign_key path %q must alternate collection ids and parent keys and end with a collection id", fk)
	}
	for _, segment := range segments {
		if segment == "" {
			return nil, fmt.Errorf("foreign_key path %q has an empty segment", fk)
		}
	}
	root, ok := ResolveForeignKey(declaringFullID, segments[0], collections)
	if !ok {
		return nil, fmt.Errorf("foreign_key path %q: root collection %q does not resolve to any collection in this definition", fk, segments[0])
	}
	path := &ForeignKeyPath{Root: root}
	declaringPath := strings.Split(declaringFullID, "/")
	targetPath := []string{root}
	current := collections[root]
	for i := 1; i < len(segments); i += 2 {
		keySpec, subID := segments[i], segments[i+1]
		step := ForeignKeyPathStep{Collection: subID}
		switch {
		case keySpec == ForeignKeySameKey:
			if !hasPrefixPath(declaringPath, targetPath) {
				return nil, fmt.Errorf("foreign_key path %q: %s after %q is only allowed where the path follows the declaring collection %q", fk, ForeignKeySameKey, strings.Join(targetPath, "/"), declaringFullID)
			}
			step.Same = true
		case strings.HasPrefix(keySpec, "{") && strings.HasSuffix(keySpec, "}"):
			step.Field = keySpec[1 : len(keySpec)-1]
			if _, declared := declaring.Columns[step.Field]; !declared {
				return nil, fmt.Errorf("foreign_key path %q: field %q is not a column of %q", fk, step.Field, declaringFullID)
			}
		default:
			step.Literal = keySpec
		}
		sub, found := current.SubCollections[subID]
		if !found {
			return nil, fmt.Errorf("foreign_key path %q: %q has no subcollection %q", fk, strings.Join(targetPath, "/"), subID)
		}
		path.Steps = append(path.Steps, step)
		targetPath = append(targetPath, subID)
		current = sub
	}
	return path, nil
}

// hasPrefixPath reports whether prefix is a leading part of path.
func hasPrefixPath(path, prefix []string) bool {
	if len(prefix) > len(path) {
		return false
	}
	for i := range prefix {
		if path[i] != prefix[i] {
			return false
		}
	}
	return true
}
