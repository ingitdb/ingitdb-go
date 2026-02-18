package config

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
	"gopkg.in/yaml.v3"
)

const RootConfigFileName = ".ingitdb.yaml"

type Language struct {
	Required string `yaml:"required,omitempty"`
	Optional string `yaml:"optional,omitempty"`
}

type RootConfig struct {
	RootCollections map[string]string `yaml:"rootCollections,omitempty"`
	Languages       []Language        `yaml:"languages,omitempty"`
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

	foundOptional := false
	for i, l := range rc.Languages {
		if l.Required != "" && l.Optional != "" {
			return fmt.Errorf("language entry at index %d cannot have both required and optional fields", i)
		}
		if l.Required == "" && l.Optional == "" {
			return fmt.Errorf("language entry at index %d must have either required or optional field", i)
		}

		langCode := l.Required
		if langCode == "" {
			langCode = l.Optional
		}

		// Basic validation for language code format (e.g., "en", "en-US", "zh-Hant-TW")
		// This regex matches simple ISO 639-1 codes and BCP 47 tags with subtags.
		// It is not exhaustive but catches obviously bad formats.
		// Regex explanation:
		// ^[a-zA-Z]{2,3} : Starts with 2 or 3 letters (primary language)
		// (-[a-zA-Z0-9]+)*$ : Optional subtags separated by hyphen
		// We can implement a simple check without pulling in a large regex library if prefered,
		// but simple string checks are efficient.
		// For simplicity/robustness without heavy deps, we'll check length and authorized chars.
		if len(langCode) < 2 {
			return fmt.Errorf("language code '%s' at index %d is too short", langCode, i)
		}
		for _, r := range langCode {
			if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '-' {
				return fmt.Errorf("language code '%s' at index %d contains invalid characters", langCode, i)
			}
		}

		if l.Required != "" {
			if foundOptional {
				return fmt.Errorf("required language '%s' at index %d must be before optional languages", l.Required, i)
			}
		} else {
			foundOptional = true
		}
	}
	return nil
}

func ReadRootConfigFromFile(dirPath string, o ingitdb.ReadOptions) (rootConfig RootConfig, err error) {
	return readRootConfigFromFile(dirPath, o, os.OpenFile)
}

func readRootConfigFromFile(dirPath string, o ingitdb.ReadOptions, openFile func(string, int, os.FileMode) (*os.File, error)) (rootConfig RootConfig, err error) {
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
	if file, err = openFile(filePath, os.O_RDONLY, 0666); err != nil {
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
