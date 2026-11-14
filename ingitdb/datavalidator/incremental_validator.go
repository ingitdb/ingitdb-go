package datavalidator

// specscore: feature/cli/validate

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/ingitdb/ingitdb-go/ingitdb"
	"github.com/ingitdb/ingitdb-go/ingitdb/config"
	"github.com/ingitdb/ingitdb-go/ingitdb/gitdiff"
)

// NewIncrementalValidator returns an IncrementalValidator that validates only
// the records whose files changed between two git refs. differ lists changed
// files, resolver maps them to collection records, and full is used as a
// fall-back when a definition file changed (the schema itself moved).
func NewIncrementalValidator(differ gitdiff.GitDiffer, resolver ChangeSetResolver, full DataValidator) IncrementalValidator {
	return &incrementalValidator{differ: differ, resolver: resolver, full: full}
}

// incrementalValidator's readDir lists directories for the name-collision
// check (os.ReadDir when nil); tests count listings through it.
type incrementalValidator struct {
	differ   gitdiff.GitDiffer
	resolver ChangeSetResolver
	full     DataValidator
	readDir  func(string) ([]os.DirEntry, error)
	// paths makes the call's pathResolver (newPathResolver when nil); tests
	// count its reads through it.
	paths func(*ingitdb.Definition) *pathResolver
}

func (iv *incrementalValidator) pathResolver(def *ingitdb.Definition) *pathResolver {
	if iv.paths != nil {
		return iv.paths(def)
	}
	return newPathResolver(def)
}

func (iv *incrementalValidator) ValidateChanges(
	ctx context.Context,
	dbPath string,
	def *ingitdb.Definition,
	fromCommit, toCommit string,
) (*ingitdb.ValidationResult, error) {
	changed, err := iv.differ.DiffFiles(ctx, dbPath, fromCommit, toCommit)
	if err != nil {
		return nil, err
	}
	// If a definition file changed, the schema itself moved — a record-scoped
	// pass could miss records that became invalid under the new schema. Fall
	// back to a full validation pass.
	if changedDefinitionFile(changed) {
		return iv.full.Validate(ctx, dbPath, def)
	}

	// One pathResolver serves the whole call, so each parent collection is
	// read once however many changed paths lie below it.
	paths := iv.pathResolver(def)
	var affected []AffectedRecord
	if r, ok := iv.resolver.(changeSetResolver); ok {
		affected = r.resolveWith(paths, dbPath, changed)
	} else if affected, err = iv.resolver.Resolve(dbPath, def, changed); err != nil {
		return nil, err
	}

	result := &ingitdb.ValidationResult{}
	names := newNameIndex(dbPath, iv.readDir)
	type counts struct{ passed, total int }
	perCollection := map[string]*counts{}
	wholeFileDone := map[string]bool{}  // collection id + shared file path → already validated
	parentNameDone := map[string]bool{} // instance data directory → its parent name already checked

	for _, ar := range affected {
		levels := ar.levels
		if levels == nil {
			levels = []instanceLevel{{id: ar.CollectionID, colDef: def.Collections[ar.CollectionID]}}
		}
		colDef := levels[len(levels)-1].colDef
		if colDef == nil {
			continue
		}
		c := perCollection[ar.CollectionID]
		if c == nil {
			c = &counts{}
			perCollection[ar.CollectionID] = c
		}
		appendErrors(result, parentNameFindings(levels, parentNameDone))
		names.useRecord(levels, ar.FilePath)
		switch colDef.RecordFile.RecordType {
		case ingitdb.SingleRecord:
			passed, total, errs := validateSingleRecordFile(ar.CollectionID, colDef, ar.FilePath)
			c.passed += passed
			c.total += total
			appendErrors(result, errs)
		case ingitdb.MapOfRecords, ingitdb.ListOfRecords:
			sharedFile := ar.CollectionID + "\x00" + collectionRecordFilePath(colDef)
			if wholeFileDone[sharedFile] {
				continue
			}
			wholeFileDone[sharedFile] = true
			passed, total, errs := validateWholeRecordFile(ar.CollectionID, colDef)
			c.passed += passed
			c.total += total
			appendErrors(result, errs)
		}
	}
	// Every changed path in a records directory counts for collisions, as
	// every entry there does in the full pass: a file that is no record (a
	// bob.YAML next to Bob.yaml), a deleted file, or a rename's old name that
	// leaves two names of one file behind.
	for _, cf := range changed {
		for _, p := range []string{cf.Path, cf.OldPath} {
			if p == "" {
				continue
			}
			absPath := filepath.Clean(filepath.Join(dbPath, p))
			if levels, _ := paths.resolve(absPath); levels != nil {
				names.useRecord(levels, absPath)
			}
		}
	}
	appendErrors(result, definitionNameFindings(names, def))
	appendErrors(result, names.findings())

	for collectionID, c := range perCollection {
		result.SetRecordCounts(collectionID, c.passed, c.total)
		result.SetRecordCount(collectionID, c.total)
	}
	return result, nil
}

// parentNameFindings reports, once per instance, the subcollection instance
// directories on the way to a changed record whose parent directory name the
// parent's key_encoding never writes, as the full pass does.
func parentNameFindings(levels []instanceLevel, done map[string]bool) []ingitdb.ValidationError {
	var findings []ingitdb.ValidationError
	for _, level := range levels[1:] {
		if level.nameErr == nil || done[level.colDef.DirPath] {
			continue
		}
		done[level.colDef.DirPath] = true
		findings = append(findings, parentNameFinding(level.id, level.colDef.DirPath, level.parentKey, level.nameErr))
	}
	return findings
}

// validateWholeRecordFile validates every record in a collection's shared
// map/list record file.
func validateWholeRecordFile(collectionKey string, colDef *ingitdb.CollectionDef) (int, int, []ingitdb.ValidationError) {
	switch colDef.RecordFile.RecordType {
	case ingitdb.MapOfRecords:
		return validateMapOfRecordsFile(collectionKey, colDef)
	case ingitdb.ListOfRecords:
		return validateListOfRecordsFile(collectionKey, colDef)
	default:
		return 0, 0, nil
	}
}

func appendErrors(result *ingitdb.ValidationResult, errs []ingitdb.ValidationError) {
	for _, e := range errs {
		result.Append(e)
	}
}

// changedDefinitionFile reports whether any changed file is a schema/definition
// file (database config, collection definition, or root-collections), in which
// case incremental validation falls back to a full pass.
func changedDefinitionFile(changed []ingitdb.ChangedFile) bool {
	for _, cf := range changed {
		base := filepath.Base(cf.Path)
		if base == ingitdb.CollectionDefFileName || base == config.RootCollectionsFileName || base == ".ingitdb.yaml" {
			return true
		}
		if strings.HasPrefix(cf.Path, config.IngitDBDirName+"/") || strings.Contains(cf.Path, "/"+config.IngitDBDirName+"/") {
			return true
		}
	}
	return false
}
