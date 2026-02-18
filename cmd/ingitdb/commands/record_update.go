package commands

import (
	"context"
	"fmt"
	"maps"

	"github.com/dal-go/dalgo/dal"
	"github.com/urfave/cli/v3"
	"gopkg.in/yaml.v3"

	"github.com/ingitdb/ingitdb-cli/pkg/dalgo2ingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// Update returns the update command group.
func Update(
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	newDB func(string, *ingitdb.Definition) (dal.DB, error),
	logf func(...any),
) *cli.Command {
	return &cli.Command{
		Name:     "update",
		Usage:    "Update database objects",
		Commands: []*cli.Command{updateRecord(homeDir, getWd, readDefinition, newDB, logf)},
	}
}

func updateRecord(
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	newDB func(string, *ingitdb.Definition) (dal.DB, error),
	logf func(...any),
) *cli.Command {
	return &cli.Command{
		Name:  "record",
		Usage: "Update fields of an existing record",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "path",
				Usage: "path to the database directory (default: current directory)",
			},
			&cli.StringFlag{
				Name:     "id",
				Usage:    "record ID in the format collection/path/key (e.g. todo/countries/ie)",
				Required: true,
			},
			&cli.StringFlag{
				Name:     "set",
				Usage:    "fields to update as YAML or JSON (e.g. '{title: \"Ireland, Republic of\"}')",
				Required: true,
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			dirPath, err := resolveDBPath(cmd, homeDir, getWd)
			if err != nil {
				return err
			}
			_ = logf

			def, err := readDefinition(dirPath)
			if err != nil {
				return fmt.Errorf("failed to read database definition: %w", err)
			}

			id := cmd.String("id")
			colDef, recordKey, err := dalgo2ingitdb.CollectionForKey(def, id)
			if err != nil {
				return fmt.Errorf("invalid --id: %w", err)
			}

			setStr := cmd.String("set")
			var patch map[string]any
			if err = yaml.Unmarshal([]byte(setStr), &patch); err != nil {
				return fmt.Errorf("failed to parse --set: %w", err)
			}

			db, err := newDB(dirPath, def)
			if err != nil {
				return fmt.Errorf("failed to open database: %w", err)
			}

			key := dal.NewKeyWithID(colDef.ID, recordKey)

			return db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
				data := map[string]any{}
				record := dal.NewRecordWithData(key, data)
				if getErr := tx.Get(ctx, record); getErr != nil {
					return getErr
				}
				if !record.Exists() {
					return fmt.Errorf("record not found: %s", id)
				}
				maps.Copy(data, patch)
				return tx.Set(ctx, record)
			})
		},
	}
}
