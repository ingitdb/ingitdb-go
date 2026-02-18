package commands

import (
	"context"
	"fmt"

	"github.com/dal-go/dalgo/dal"
	"github.com/urfave/cli/v3"

	"github.com/ingitdb/ingitdb-cli/pkg/dalgo2ingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

func deleteRecord(
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	newDB func(string, *ingitdb.Definition) (dal.DB, error),
	logf func(...any),
) *cli.Command {
	return &cli.Command{
		Name:  "record",
		Usage: "Delete a single record by its ID",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "path",
				Usage: "path to the database directory (default: current directory)",
			},
			&cli.StringFlag{
				Name:     "id",
				Usage:    "record ID in the format collection/path/key (e.g. todo/tags/ie)",
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

			db, err := newDB(dirPath, def)
			if err != nil {
				return fmt.Errorf("failed to open database: %w", err)
			}

			key := dal.NewKeyWithID(colDef.ID, recordKey)

			return db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
				return tx.Delete(ctx, key)
			})
		},
	}
}
