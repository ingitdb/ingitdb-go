package ingitdb

import (
	"fmt"
	"path/filepath"
)

func Validate(dir string) error {
	rootConfig, err := validateRootConfig(dir)
	if err != nil {
		return fmt.Errorf("failed to validate root config file %s: %v", RootConfigFileName, err)
	}
	return validateRootCollections(rootConfig)
}

func validateRootConfig(dir string) (rootConfig *RootConfig, err error) {
	rootConfig, err = ReadRootConfigFromFile(filepath.Join(dir, RootConfigFileName))
	if err != nil {
		return
	}
	if err = rootConfig.Validate(); err != nil {
		return nil, fmt.Errorf("content of root config is not valid: %w", err)
	}
	return
}

func validateRootCollections(rootConfig *RootConfig) (err error) {
	for id, path := range rootConfig.RootCollections {
		if err = validateCollectionDef(id, path); err != nil {
			return fmt.Errorf("failed to validate root collection def ID=%s: %w", id, err)
		}
	}
	return
}

func validateCollectionDef(id string, path string) error {
	_, _ = id, path
	return nil
}
