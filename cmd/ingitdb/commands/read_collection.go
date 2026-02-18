package commands

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/urfave/cli/v3"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

func readCollection(
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	logf func(...any),
) *cli.Command {
	return &cli.Command{
		Name:  "collection",
		Usage: "Output the definition YAML of a collection",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "path",
				Usage: "path to the database directory (default: current directory)",
			},
			&cli.StringFlag{
				Name:     "collection",
				Usage:    "collection ID using slash or dot separator (e.g. countries or todo/countries)",
				Required: true,
			},
		},
		Action: func(_ context.Context, cmd *cli.Command) error {
			dirPath, err := resolveDBPath(cmd, homeDir, getWd)
			if err != nil {
				return err
			}
			_ = logf

			def, err := readDefinition(dirPath)
			if err != nil {
				return fmt.Errorf("failed to read database definition: %w", err)
			}

			wantNorm := strings.ReplaceAll(cmd.String("collection"), ".", "/")
			var colDef *ingitdb.CollectionDef
			for colID, cd := range def.Collections {
				if strings.ReplaceAll(colID, ".", "/") == wantNorm {
					colDef = cd
					break
				}
			}
			if colDef == nil {
				return fmt.Errorf("collection %q not found", cmd.String("collection"))
			}

			defPath := filepath.Join(colDef.DirPath, ingitdb.CollectionDefFileName)
			content, readErr := os.ReadFile(defPath)
			if readErr != nil {
				return fmt.Errorf("failed to read collection definition file: %w", readErr)
			}
			_, _ = os.Stdout.Write(content)
			return nil
		},
	}
}
