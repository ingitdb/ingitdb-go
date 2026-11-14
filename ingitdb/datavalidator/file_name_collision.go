package datavalidator

// specscore: feature/record-key-encoding

import (
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"

	ingitdb "github.com/ingitdb/ingitdb-go/ingitdb"
)

// collisionReason describes a group of names that ingitdb.CaseOnlyDifferences
// found to be one entry: the names as a message shows them (escaped when they
// differ in Unicode normalisation, which prints the same), and why they are
// one noun ("file", "directory") and on which file systems.
func collisionReason(names []string, noun string) ([]string, string) {
	decomposed := map[string]bool{}
	folded := map[string]bool{}
	fold := cases.Fold()
	for _, name := range names {
		decomposed[norm.NFD.String(name)] = true
		folded[fold.String(name)] = true
	}
	inCase, inNormalisation := len(decomposed) > 1, len(folded) > 1
	shown := slices.Clone(names)
	if inNormalisation {
		for i, name := range shown {
			shown[i] = strconv.QuoteToASCII(name)
		}
	}
	switch {
	case inCase && inNormalisation:
		return shown, "differ only in letter case and Unicode normalisation: they are the same " + noun + " on macOS, whose file systems ignore both by default"
	case inNormalisation:
		return shown, "differ only in Unicode normalisation (NFC/NFD): they are the same " + noun + " on macOS, whose file systems ignore normalisation"
	default:
		return shown, "differ only in letter case: they are the same " + noun + " on case-insensitive file systems such as macOS and Windows"
	}
}

// collectionDirCollisionFindings reports declared collection directories
// that are one directory on a case-insensitive or normalisation-insensitive
// file system: root collections whose data directories share a parent, and
// the subcollections of one collection, whose directories sit side by side
// under every parent record. They come from the definition, so incremental
// validation, which falls back to this full pass when a definition changes,
// never needs them.
func collectionDirCollisionFindings(def *ingitdb.Definition) []ingitdb.ValidationError {
	var findings []ingitdb.ValidationError
	byParent := map[string][]string{}
	idOf := map[string]string{}
	for _, id := range slices.Sorted(maps.Keys(def.Collections)) {
		if colDef := def.Collections[id]; colDef != nil && colDef.DirPath != "" {
			dir := colDef.DirPath
			byParent[filepath.Dir(dir)] = append(byParent[filepath.Dir(dir)], filepath.Base(dir))
			idOf[dir] = id
		}
	}
	for _, parent := range slices.Sorted(maps.Keys(byParent)) {
		for _, group := range ingitdb.CaseOnlyDifferences(byParent[parent]) {
			shown, reason := collisionReason(group, "directory")
			findings = append(findings, newValidationError(idOf[filepath.Join(parent, group[0])], filepath.Join(parent, group[0]), "", "",
				fmt.Sprintf("collection directories %s in %s %s; a checkout there mixes their records: rename all but one", strings.Join(shown, ", "), parent, reason), nil))
		}
	}
	for _, id := range slices.Sorted(maps.Keys(def.Collections)) {
		findings = append(findings, subCollectionDirCollisionFindings(id, def.Collections[id])...)
	}
	return findings
}

// subCollectionDirCollisionFindings reports, recursively, subcollection ids
// of one collection that name one directory.
func subCollectionDirCollisionFindings(fullID string, colDef *ingitdb.CollectionDef) []ingitdb.ValidationError {
	if colDef == nil {
		return nil
	}
	ids := slices.Sorted(maps.Keys(colDef.SubCollections))
	var findings []ingitdb.ValidationError
	for _, group := range ingitdb.CaseOnlyDifferences(ids) {
		shown, reason := collisionReason(group, "directory")
		findings = append(findings, newValidationError(fullID+"/"+group[0], "", "", "",
			fmt.Sprintf("subcollections %s of %s: their directories %s; under every parent record a checkout there mixes their records: rename all but one", strings.Join(shown, ", "), fullID, reason), nil))
	}
	for _, id := range ids {
		findings = append(findings, subCollectionDirCollisionFindings(fullID+"/"+id, colDef.SubCollections[id])...)
	}
	return findings
}
