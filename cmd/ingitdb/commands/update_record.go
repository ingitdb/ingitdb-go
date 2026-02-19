package commands

import (
	"context"
	"fmt"
	"maps"

	"github.com/dal-go/dalgo/dal"
	"github.com/urfave/cli/v3"
	"gopkg.in/yaml.v3"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

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
				Name:  "github",
				Usage: "GitHub source as owner/repo[@branch|tag|commit]",
			},
			&cli.StringFlag{
				Name:  "token",
				Usage: "GitHub personal access token (or set GITHUB_TOKEN env var)",
			},
			&cli.StringFlag{
				Name:     "id",
				Usage:    "record ID in the format collection/path/key (e.g. todo.countries/ie)",
				Required: true,
			},
			&cli.StringFlag{
				Name:     "set",
				Usage:    "fields to update as YAML or JSON (e.g. '{title: \"Ireland, Republic of\"}')",
				Required: true,
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			_ = logf
			id := cmd.String("id")
			setStr := cmd.String("set")
			rctx, err := resolveRecordContext(ctx, cmd, id, homeDir, getWd, readDefinition, newDB)
			if err != nil {
				return err
			}
			var patch map[string]any
			if unmarshalErr := yaml.Unmarshal([]byte(setStr), &patch); unmarshalErr != nil {
				return fmt.Errorf("failed to parse --set: %w", unmarshalErr)
			}
			key := dal.NewKeyWithID(rctx.colDef.ID, rctx.recordKey)
			err = rctx.db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
				data := map[string]any{}
				record := dal.NewRecordWithData(key, data)
				getErr := tx.Get(ctx, record)
				if getErr != nil {
					return getErr
				}
				if !record.Exists() {
					return fmt.Errorf("record not found: %s", id)
				}
				maps.Copy(data, patch)
				return tx.Set(ctx, record)
			})
			if err != nil {
				return err
			}
			return buildLocalViews(ctx, rctx)
		},
	}
}
