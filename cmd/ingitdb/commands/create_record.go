package commands

import (
	"context"
	"fmt"

	"github.com/dal-go/dalgo/dal"
	"github.com/urfave/cli/v3"
	"gopkg.in/yaml.v3"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

func createRecord(
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	newDB func(string, *ingitdb.Definition) (dal.DB, error),
	logf func(...any),
) *cli.Command {
	return &cli.Command{
		Name:  "record",
		Usage: "Create a new record in a collection",
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
				Name:     "data",
				Usage:    "record data as YAML or JSON (e.g. '{title: \"Ireland\"}')",
				Required: true,
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			id := cmd.String("id")
			dataStr := cmd.String("data")
			rctx, err := resolveRecordContext(ctx, cmd, id, homeDir, getWd, readDefinition, newDB)
			if err != nil {
				return err
			}
			if rctx.dirPath != "" {
				logf("inGitDB db path: ", rctx.dirPath)
			}
			var data map[string]any
			if unmarshalErr := yaml.Unmarshal([]byte(dataStr), &data); unmarshalErr != nil {
				return fmt.Errorf("failed to parse --data: %w", unmarshalErr)
			}
			key := dal.NewKeyWithID(rctx.colDef.ID, rctx.recordKey)
			record := dal.NewRecordWithData(key, data)
			err = rctx.db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
				return tx.Insert(ctx, record)
			})
			if err != nil {
				return err
			}
			return buildLocalViews(ctx, rctx)
		},
	}
}
