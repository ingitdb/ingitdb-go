package ingitdb

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

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

func ReadRootConfigFromFile(dirPath string) (rootConfig *RootConfig, err error) {
	if dirPath == "" {
		dirPath = "."
	}
	filePath := filepath.Join(dirPath, RootConfigFileName)

	var file *os.File
	if file, err = os.OpenFile(filePath, os.O_RDONLY, 0666); err != nil {
		return nil, fmt.Errorf("failed to open root config file: %w", err)
	}

	if err = yaml.NewDecoder(file).Decode(RootConfig{}); err != nil {
		return nil, fmt.Errorf("failed to parse root config: %w", err)
	}
	return nil, nil
}
