package config

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/ingitdb/ingitdb-go/ingitdb"
	"gopkg.in/yaml.v3"
)

const RootConfigFileName = ".ingitdb.yaml"

type RootConfig struct {
	RootCollections map[string]string `yaml:"rootCollections,omitempty"`
}

func (rc *RootConfig) Validate() error {
	if rc == nil {
		return nil
	}
	var paths []string
	for id, path := range rc.RootCollections {
		if id == "" {
			return errors.New("root collection id cannot be empty")
		}
		if path == "" {
			return fmt.Errorf("root collection path cannot be empty, ID=%s", id)
		}
		for _, p := range paths {
			if p == path {
				return fmt.Errorf("duplicate path for ID=%s: %s", id, p)
			}
		}
		paths = append(paths, path)
	}
	return nil
}

func ReadRootConfigFromFile(dirPath string, o ingitdb.ReadOptions) (rootConfig RootConfig, err error) {
	defer func() {
		r := recover()
		if r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()
	if dirPath == "" {
		dirPath = "."
	}
	filePath := filepath.Join(dirPath, RootConfigFileName)

	var file *os.File
	if file, err = os.OpenFile(filePath, os.O_RDONLY, 0666); err != nil {
		err = fmt.Errorf("failed to open root config file: %w", err)
		return
	}

	decoder := yaml.NewDecoder(file)
	decoder.KnownFields(true)

	if err = decoder.Decode(&rootConfig); err != nil {
		err = fmt.Errorf("failed to parse root config file: %w\nNote: Expected keys in .ingitdb.yaml include 'rootCollections'", err)
		return
	}

	if o.IsValidationRequired() {
		if err = rootConfig.Validate(); err != nil {
			return rootConfig, fmt.Errorf("content of root config is not valid: %w", err)
		}
		log.Println(".ingitdb.yaml is valid")
	}
	return
}
