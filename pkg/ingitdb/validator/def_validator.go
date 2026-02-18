package validator

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb/config"
	"gopkg.in/yaml.v3"
)

// definitionReader wraps ReadDefinition to satisfy ingitdb.CollectionsReader.
type definitionReader struct{}

// NewCollectionsReader returns an ingitdb.CollectionsReader backed by ReadDefinition.
func NewCollectionsReader() ingitdb.CollectionsReader { return definitionReader{} }

func (definitionReader) ReadDefinition(dbPath string, opts ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
	return ReadDefinition(dbPath, opts...)
}

func ReadDefinition(rootPath string, o ...ingitdb.ReadOption) (def *ingitdb.Definition, err error) {
	opts := ingitdb.NewReadOptions(o...)
	var rootConfig config.RootConfig
	rootConfig, err = config.ReadRootConfigFromFile(rootPath, opts)
	if err != nil {
		err = fmt.Errorf("failed to read root config file %s: %v", config.RootConfigFileName, err)
		return
	}
	return readRootCollections(rootPath, rootConfig, opts)
}

func readRootCollections(rootPath string, rootConfig config.RootConfig, o ingitdb.ReadOptions) (def *ingitdb.Definition, err error) {
	def = new(ingitdb.Definition)
	def.Collections = make(map[string]*ingitdb.CollectionDef)
	for id, colPath := range rootConfig.RootCollections {
		if strings.HasSuffix(colPath, "*") {
			_, err = readCollectionDefs(def, rootPath, colPath, id, o)
			if err != nil {
				err = fmt.Errorf("failed to validate root collections def (%s @ %s): %w", id, colPath, err)
				return
			}
			//log.Printf("Definition of root collections is valid (%s @ %s)", id, colPath)
		} else {
			var colDef *ingitdb.CollectionDef
			// For single collection, colPath is the colPath to the directory containing the collection directory
			// or the colPath is the directory itself?
			// In readCollectionDef: filepath.Join(colPath, id, collectionDefFileName)
			// So if id="countries" and colPath="geo", it looks for geo/countries/.ingitdb-collection.yaml
			if colDef, err = readCollectionDef(rootPath, colPath, id, o); err != nil {
				err = fmt.Errorf("failed to validate root collection def ID=%s: %w", id, err)
				return
			}
			def.Collections[id] = colDef
			//log.Printf("Definition of root collections '%s' is valid - %s", id, colPath)
		}
	}
	return
}

func readCollectionDef(rootPath, relPath, id string, o ingitdb.ReadOptions) (colDef *ingitdb.CollectionDef, err error) {
	colDefFilePath := filepath.Join(rootPath, relPath, ingitdb.CollectionDefFileName)
	var fileContent []byte
	fileContent, err = os.ReadFile(colDefFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", colDefFilePath, err)
	}
	//log.Println(string(fileContent))
	colDef = new(ingitdb.CollectionDef)

	err = yaml.Unmarshal(fileContent, colDef)
	if err != nil {
		return nil, fmt.Errorf("failed to parse YAML file %s: %w", colDefFilePath, err)
	}
	colDef.ID = id
	colDef.DirPath = filepath.Join(rootPath, relPath)

	if o.IsValidationRequired() {
		if err = colDef.Validate(); err != nil {
			err = fmt.Errorf("not valid definition of collection '%s': %w", id, err)
			return
		}
		log.Printf("Definition of collection '%s' is valid", colDef.ID)
	}
	return
}

func readCollectionDefs(def *ingitdb.Definition, rootPath, relPath, idPrefix string, o ingitdb.ReadOptions) (colDefs []*ingitdb.CollectionDef, err error) {
	relPath = strings.TrimSuffix(relPath, "*")
	dirPath := filepath.Join(rootPath, relPath)
	var entries []os.DirEntry
	entries, err = os.ReadDir(dirPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read dir %s: %w", dirPath, err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		var colDef *ingitdb.CollectionDef
		localID := entry.Name()
		var globalID string
		if strings.HasSuffix(idPrefix, ".") {
			globalID = idPrefix + entry.Name()
		} else {
			globalID = idPrefix + "." + entry.Name()
		}
		colRelPath := filepath.Join(relPath, localID)
		if colDef, err = readCollectionDef(rootPath, colRelPath, globalID, o); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			err = fmt.Errorf("failed to read collection def '%s': %w", localID, err)
			return
		}
		colDefs = append(colDefs, colDef)
		def.Collections[globalID] = colDef
	}
	return
}
