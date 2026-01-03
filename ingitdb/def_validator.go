package ingitdb

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

func Validate(rootPath string) error {
	rootConfig, err := validateRootConfig(rootPath)
	if err != nil {
		return fmt.Errorf("failed to validate root config file %s: %v", RootConfigFileName, err)
	}
	return validateRootCollections(rootPath, rootConfig)
}

func validateRootConfig(dir string) (rootConfig RootConfig, err error) {
	rootConfig, err = ReadRootConfigFromFile(dir)
	if err != nil {
		return rootConfig, fmt.Errorf("failed to read root config file %s: %v", RootConfigFileName, err)
	}
	if err = rootConfig.Validate(); err != nil {
		return rootConfig, fmt.Errorf("content of root config is not valid: %w", err)
	}
	log.Println("/.ingitdb.yaml is valid")
	return
}

func validateRootCollections(rootPath string, rootConfig RootConfig) (err error) {
	var def Definition
	def.Collections = make(map[string]*CollectionDef)
	for id, colPath := range rootConfig.RootCollections {
		if strings.HasSuffix(colPath, "*") {
			if err = validateCollectionDefs(rootPath, colPath, id, &def); err != nil {
				return fmt.Errorf("failed to validate root collection def ID=%s: %w", id, err)
			}
			log.Printf("Definition of root collection '%s' is valid - %s", id, colPath)
		} else {
			var colDef *CollectionDef
			// For single collection, colPath is the colPath to the directory containing the collection directory
			// or the colPath is the directory itself?
			// In readCollectionDef: filepath.Join(colPath, id, collectionDefFileName)
			// So if id="countries" and colPath="geo", it looks for geo/countries/.ingitdb-collection.yaml
			if colDef, err = validateCollectionDef(rootPath, colPath, id); err != nil {
				return fmt.Errorf("failed to validate root collection def ID=%s: %w", id, err)
			}
			def.Collections[id] = colDef
			log.Printf("Definition of root collections '%s' is valid - %s", id, colPath)
		}
	}
	return
}

func validateCollectionDef(rootPath, relPath, id string) (colDef *CollectionDef, err error) {
	if colDef, err = readCollectionDef(rootPath, relPath, id); err != nil {
		err = fmt.Errorf("failed to read definition of collection '%s': %w", id, err)
		return
	}
	if err = colDef.Validate(); err != nil {
		err = fmt.Errorf("not valid definition of collection '%s': %w", id, err)
		return
	}
	return
}

func validateCollectionDefs(rootPath, path, id string, def *Definition) error {
	colDefs, err := readCollectionDefs(rootPath, path)
	if err != nil {
		return fmt.Errorf("failed to read collection definitions for '%s' at %s: %w", id, path, err)
	}
	for _, colDef := range colDefs {
		if err = colDef.Validate(); err != nil {
			return fmt.Errorf("failed to validate collection definition for '%s' at %s: %w", colDef.ID, path, err)
		}
		def.Collections[colDef.ID] = colDef
		log.Println(fmt.Sprintf("Definition of collections '%s' is valid", colDef.ID))
	}
	return nil
}

func readCollectionDef(rootPath, relPath, id string) (colDef *CollectionDef, err error) {
	colDefFilePath := filepath.Join(rootPath, relPath, collectionDefFileName)
	var fileContent []byte
	fileContent, err = os.ReadFile(colDefFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", colDefFilePath, err)
	}
	//log.Println(string(fileContent))
	colDef = new(CollectionDef)

	err = yaml.Unmarshal(fileContent, colDef)
	if err != nil {
		return nil, fmt.Errorf("failed to parse YAML file %s: %w", colDefFilePath, err)
	}
	colDef.ID = id
	return
}

func readCollectionDefs(rootPath, relPath string) (colDefs []*CollectionDef, err error) {
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
		var colDef *CollectionDef
		colID := entry.Name()
		colRelPath := filepath.Join(relPath, colID)
		if colDef, err = readCollectionDef(rootPath, colRelPath, colID); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return
		}
		colDefs = append(colDefs, colDef)
	}
	return
}
