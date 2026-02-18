package commands

import (
	"context"
	"fmt"

	"github.com/dal-go/dalgo/dal"
	"github.com/urfave/cli/v3"

	"github.com/ingitdb/ingitdb-cli/pkg/dalgo2ingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// Delete returns the delete command.
func Delete(
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	newDB func(string, *ingitdb.Definition) (dal.DB, error),
	logf func(...any),
) *cli.Command {
	return &cli.Command{
		Name:     "delete",
		Usage:    "Delete database objects (collection, view, or records)",
		Commands: []*cli.Command{collection(), deleteView(), records(), deleteRecord(homeDir, getWd, readDefinition, newDB, logf)},
	}
}

func collection() *cli.Command {
	return &cli.Command{
		Name:  "collection",
		Usage: "Delete a collection and all its records",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "path",
				Usage: "path to the database directory",
			},
			&cli.StringFlag{
				Name:     "collection",
				Usage:    "collection id to delete (e.g. countries/ie/counties)",
				Required: true,
			},
		},
		Action: func(_ context.Context, _ *cli.Command) error {
			return cli.Exit("not yet implemented", 1)
		},
	}
}

// deleteView is named with the parent prefix because "view" also appears as a
// subcommand of list.
func deleteView() *cli.Command {
	return &cli.Command{
		Name:  "view",
		Usage: "Delete a view definition and its materialised files",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "path",
				Usage: "path to the database directory",
			},
			&cli.StringFlag{
				Name:     "view",
				Usage:    "view id to delete",
				Required: true,
			},
		},
		Action: func(_ context.Context, _ *cli.Command) error {
			return cli.Exit("not yet implemented", 1)
		},
	}
}

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

func records() *cli.Command {
	return &cli.Command{
		Name:  "records",
		Usage: "Delete individual records from a collection",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "path",
				Usage: "path to the database directory",
			},
			&cli.StringFlag{
				Name:     "collection",
				Usage:    "collection to delete records from",
				Required: true,
			},
			&cli.StringFlag{
				Name:  "in",
				Usage: "regular expression scoping deletion to a sub-path",
			},
			&cli.StringFlag{
				Name:  "filter-name",
				Usage: "pattern to match record names to delete",
			},
		},
		Action: func(_ context.Context, _ *cli.Command) error {
			return cli.Exit("not yet implemented", 1)
		},
	}
}
