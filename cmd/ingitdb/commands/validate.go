package commands

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/urfave/cli/v3"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb/datavalidator"
)

// Validate returns the validate command.
func Validate(
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	dataVal datavalidator.DataValidator,
	incVal datavalidator.IncrementalValidator,
	logf func(...any),
) *cli.Command {
	return &cli.Command{
		Name:  "validate",
		Usage: "Validate an inGitDB database directory",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "path",
				Usage: "path to the database directory (default: current directory)",
			},
			&cli.StringFlag{
				Name:  "from-commit",
				Usage: "validate only records changed since this commit",
			},
			&cli.StringFlag{
				Name:  "to-commit",
				Usage: "validate only records up to this commit",
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			dirPath := cmd.String("path")
			if dirPath == "" {
				wd, err := getWd()
				if err != nil {
					return fmt.Errorf("failed to get working directory: %w", err)
				}
				dirPath = wd
			}
			expanded, err := expandHome(dirPath, homeDir)
			if err != nil {
				return err
			}
			dirPath = expanded
			logf("inGitDB db path: ", dirPath)

			fromCommit := cmd.String("from-commit")
			toCommit := cmd.String("to-commit")

			if fromCommit != "" || toCommit != "" {
				if incVal == nil {
					return fmt.Errorf("incremental validation (--from-commit/--to-commit) is not yet implemented")
				}
				def, defErr := readDefinition(dirPath)
				if defErr != nil {
					return fmt.Errorf("failed to read database definition: %w", defErr)
				}
				result, valErr := incVal.ValidateChanges(ctx, dirPath, def, fromCommit, toCommit)
				if valErr != nil {
					return fmt.Errorf("incremental validation failed: %w", valErr)
				}
				if result.HasErrors() {
					errCount := result.ErrorCount()
					return fmt.Errorf("incremental validation found %d error(s)", errCount)
				}
				return nil
			}

			validateOpt := ingitdb.Validate()
			def, err := readDefinition(dirPath, validateOpt)
			if err != nil {
				return fmt.Errorf("inGitDB database validation failed: %w", err)
			}
			if dataVal != nil {
				result, valErr := dataVal.Validate(ctx, dirPath, def)
				if valErr != nil {
					return fmt.Errorf("data validation failed: %w", valErr)
				}
				if result.HasErrors() {
					errCount := result.ErrorCount()
					return fmt.Errorf("data validation found %d error(s)", errCount)
				}
			}
			return nil
		},
	}
}

func expandHome(path string, homeDir func() (string, error)) (string, error) {
	if strings.HasPrefix(path, "~") {
		home, err := homeDir()
		if err != nil {
			return "", fmt.Errorf("failed to expand home directory: %w", err)
		}
		return filepath.Join(home, path[1:]), nil
	}
	return path, nil
}
