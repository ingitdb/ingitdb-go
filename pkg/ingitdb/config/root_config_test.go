package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ingitdb/ingitdb-go/pkg/ingitdb"
)

func TestRootConfigValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		rc   *RootConfig
		err  string
	}{
		{
			name: "nil_receiver",
			rc:   nil,
			err:  "",
		},
		{
			name: "empty_id",
			rc: &RootConfig{
				RootCollections: map[string]string{
					"": "path",
				},
			},
			err: "root collection id cannot be empty",
		},
		{
			name: "empty_path",
			rc: &RootConfig{
				RootCollections: map[string]string{
					"foo": "",
				},
			},
			err: "root collection path cannot be empty",
		},
		{
			name: "duplicate_path",
			rc: &RootConfig{
				RootCollections: map[string]string{
					"foo": "same",
					"bar": "same",
				},
			},
			err: "duplicate path",
		},
		{
			name: "valid_languages",
			rc: &RootConfig{
				RootCollections: map[string]string{"foo": "bar"},
				Languages: []Language{
					{Required: "en"},
					{Required: "fr"},
					{Optional: "es"},
				},
			},
			err: "",
		},
		{
			name: "invalid_languages_both_set",
			rc: &RootConfig{
				RootCollections: map[string]string{"foo": "bar"},
				Languages: []Language{
					{Required: "en", Optional: "es"},
				},
			},
			err: "cannot have both required and optional fields",
		},
		{
			name: "invalid_languages_neither_set",
			rc: &RootConfig{
				RootCollections: map[string]string{"foo": "bar"},
				Languages: []Language{
					{},
				},
			},
			err: "must have either required or optional field",
		},
		{
			name: "invalid_languages_order",
			rc: &RootConfig{
				RootCollections: map[string]string{"foo": "bar"},
				Languages: []Language{
					{Optional: "en"},
					{Required: "fr"},
				},
			},
			err: "must be before optional languages",
		},
		{
			name: "invalid_languages_code_short",
			rc: &RootConfig{
				RootCollections: map[string]string{"foo": "bar"},
				Languages: []Language{
					{Required: "a"},
				},
			},
			err: "too short",
		},
		{
			name: "invalid_languages_code_chars",
			rc: &RootConfig{
				RootCollections: map[string]string{"foo": "bar"},
				Languages: []Language{
					{Required: "en$US"},
				},
			},
			err: "contains invalid characters",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.rc.Validate()

			if tt.err == "" && err != nil {
				errMsg := err.Error()
				t.Fatalf("expected no error, got %s", errMsg)
			}
			if tt.err != "" {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				errMsg := err.Error()
				if !strings.Contains(errMsg, tt.err) {
					t.Fatalf("expected error to contain %q, got %q", tt.err, errMsg)
				}
			}
		})
	}
}

func TestReadRootConfigFromFile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		setup         func(dir string) error
		options       ingitdb.ReadOptions
		dirPath       string
		useDirPath    bool
		expectedError string
		verify        func(t *testing.T, rc RootConfig)
	}{
		{
			name:          "missing_file",
			options:       ingitdb.NewReadOptions(),
			expectedError: "failed to open root config file",
		},
		{
			name:          "empty_dir_path",
			options:       ingitdb.NewReadOptions(),
			dirPath:       "",
			useDirPath:    true,
			expectedError: "failed to open root config file",
		},
		{
			name: "unknown_field",
			setup: func(dir string) error {
				filePath := filepath.Join(dir, RootConfigFileName)
				content := []byte("unknown: value\n")
				return os.WriteFile(filePath, content, 0666)
			},
			options:       ingitdb.NewReadOptions(),
			expectedError: "failed to parse root config file",
		},
		{
			name: "invalid_content_with_validation",
			setup: func(dir string) error {
				filePath := filepath.Join(dir, RootConfigFileName)
				content := []byte("rootCollections:\n  \"\": \"path\"\n")
				return os.WriteFile(filePath, content, 0666)
			},
			options:       ingitdb.NewReadOptions(ingitdb.Validate()),
			expectedError: "content of root config is not valid",
		},
		{
			name: "valid_content_with_validation",
			setup: func(dir string) error {
				filePath := filepath.Join(dir, RootConfigFileName)
				content := []byte("rootCollections:\n  countries: \"geo/countries\"\n")
				return os.WriteFile(filePath, content, 0666)
			},
			options:       ingitdb.NewReadOptions(ingitdb.Validate()),
			expectedError: "",
		},
		{
			name: "valid_languages_yaml",
			setup: func(dir string) error {
				filePath := filepath.Join(dir, RootConfigFileName)
				content := []byte(`
languages:
  - required: en
  - optional: fr
`)
				return os.WriteFile(filePath, content, 0666)
			},
			options:       ingitdb.NewReadOptions(ingitdb.Validate()),
			expectedError: "",
			verify: func(t *testing.T, rc RootConfig) {
				if len(rc.Languages) != 2 {
					t.Fatalf("expected 2 languages, got %d", len(rc.Languages))
				}
				if rc.Languages[0].Required != "en" {
					t.Errorf("expected first language required=en, got %s", rc.Languages[0].Required)
				}
				if rc.Languages[1].Optional != "fr" {
					t.Errorf("expected second language optional=fr, got %s", rc.Languages[1].Optional)
				}
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			if tt.setup != nil {
				err := tt.setup(dir)
				if err != nil {
					errMsg := err.Error()
					t.Fatalf("failed to setup test data: %s", errMsg)
				}
			}

			dirPath := dir
			if tt.useDirPath {
				dirPath = tt.dirPath
			}

			rc, err := ReadRootConfigFromFile(dirPath, tt.options)
			if tt.expectedError == "" && err != nil {
				errMsg := err.Error()
				t.Fatalf("expected no error, got %s", errMsg)
			}
			if tt.expectedError != "" {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				errMsg := err.Error()
				if !strings.Contains(errMsg, tt.expectedError) {
					t.Fatalf("expected error to contain %q, got %q", tt.expectedError, errMsg)
				}
			}
			if tt.verify != nil {
				tt.verify(t, rc)
			}
		})
	}
}

func TestReadRootConfigFromFile_PanicRecovery(t *testing.T) {
	t.Parallel()

	openFile := func(string, int, os.FileMode) (*os.File, error) {
		panic("boom")
	}

	_, err := readRootConfigFromFile("irrelevant", ingitdb.NewReadOptions(), openFile)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	errMsg := err.Error()
	if !strings.Contains(errMsg, "panic: boom") {
		t.Fatalf("expected panic error, got %s", errMsg)
	}
}
