package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ingitdb/ingitdb-go/ingitdb"
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
		expectedError string
	}{
		{
			name:          "missing_file",
			options:       ingitdb.NewReadOptions(),
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

			_, err := ReadRootConfigFromFile(dir, tt.options)
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
		})
	}
}
