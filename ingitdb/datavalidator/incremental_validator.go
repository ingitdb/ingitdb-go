package datavalidator

// specscore: feature/cli/validate

import (
	"context"
	"fmt"
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

type incrementalValidator struct {
	differ   gitdiff.GitDiffer
	resolver ChangeSetResolver
	full     DataValidator
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

	affected, err := iv.resolver.Resolve(dbPath, def, changed)
	if err != nil {
		return nil, err
	}

	result := &ingitdb.ValidationResult{}
	type counts struct{ passed, total int }
	perCollection := map[string]*counts{}
	wholeFileDone := map[string]bool{} // collectionID → its shared file already validated

	for _, ar := range affected {
		colDef := def.Collections[ar.CollectionID]
		if colDef == nil {
			continue
		}
		c := perCollection[ar.CollectionID]
		if c == nil {
			c = &counts{}
			perCollection[ar.CollectionID] = c
		}
		switch colDef.RecordFile.RecordType {
		case ingitdb.SingleRecord:
			passed, total, errs := validateSingleRecordFile(ar.CollectionID, colDef, ar.FilePath)
			c.passed += passed
			c.total += total
			appendErrors(result, errs)
			appendErrors(result, changedFileCaseFindings(dbPath, ar.CollectionID, colDef, ar.FilePath))
		case ingitdb.MapOfRecords, ingitdb.ListOfRecords:
			if wholeFileDone[ar.CollectionID] {
				continue
			}
			wholeFileDone[ar.CollectionID] = true
			passed, total, errs := validateWholeRecordFile(ar.CollectionID, colDef)
			c.passed += passed
			c.total += total
			appendErrors(result, errs)
		}
	}

	for collectionID, c := range perCollection {
		result.SetRecordCounts(collectionID, c.passed, c.total)
		result.SetRecordCount(collectionID, c.total)
	}
	return result, nil
}

// changedFileCaseFindings reports a changed record file of a collection that
// declares a key encoding when its name, or a directory on its path, differs
// from an existing entry only in letter case.
func changedFileCaseFindings(dbPath, collectionID string, colDef *ingitdb.CollectionDef, filePath string) []ingitdb.ValidationError {
	if !colDef.RecordFile.EncodesKeys() {
		return nil
	}
	existing, found := ingitdb.PathDifferingInCase(dbPath, filePath)
	if !found {
		return nil
	}
	message := fmt.Sprintf("%s and %s differ only in letter case: on a case-insensitive file system (Windows, macOS by default) they are one entry; give the records keys that differ in more than case", filePath, existing)
	return []ingitdb.ValidationError{newValidationError(collectionID, filePath, "", "", message, nil)}
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
