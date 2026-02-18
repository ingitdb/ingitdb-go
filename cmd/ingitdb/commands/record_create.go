package commands

import (
	"context"
	"fmt"

	"github.com/dal-go/dalgo/dal"
	"github.com/urfave/cli/v3"
	"gopkg.in/yaml.v3"

	"github.com/ingitdb/ingitdb-cli/pkg/dalgo2ingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// Create returns the create command for inserting a single record.
func Create(
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	newDB func(string, *ingitdb.Definition) (dal.DB, error),
	logf func(...any),
) *cli.Command {
	return &cli.Command{
		Name:  "create",
		Usage: "Create a new record in a collection",
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
			&cli.StringFlag{
				Name:     "data",
				Usage:    "record data as YAML or JSON (e.g. '{title: \"Ireland\"}')",
				Required: true,
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			dirPath, err := resolveDBPath(cmd, homeDir, getWd)
			if err != nil {
				return err
			}
			logf("inGitDB db path: ", dirPath)

			def, err := readDefinition(dirPath)
			if err != nil {
				return fmt.Errorf("failed to read database definition: %w", err)
			}

			id := cmd.String("id")
			colDef, recordKey, err := dalgo2ingitdb.CollectionForKey(def, id)
			if err != nil {
				return fmt.Errorf("invalid --id: %w", err)
			}

			dataStr := cmd.String("data")
			var data map[string]any
			if err = yaml.Unmarshal([]byte(dataStr), &data); err != nil {
				return fmt.Errorf("failed to parse --data: %w", err)
			}

			db, err := newDB(dirPath, def)
			if err != nil {
				return fmt.Errorf("failed to open database: %w", err)
			}

			key := dal.NewKeyWithID(colDef.ID, recordKey)
			record := dal.NewRecordWithData(key, data)

			return db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
				return tx.Insert(ctx, record)
			})
		},
	}
}
