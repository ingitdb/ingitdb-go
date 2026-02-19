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
				Name:  "github",
				Usage: "GitHub source as owner/repo[@branch|tag|commit]",
			},
			&cli.StringFlag{
				Name:  "token",
				Usage: "GitHub personal access token (or set GITHUB_TOKEN env var)",
			},
			&cli.StringFlag{
				Name:     "id",
				Usage:    "record ID in the format collection/path/key (e.g. todo.tags/ie)",
				Required: true,
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			id := cmd.String("id")
			githubValue := cmd.String("github")
			var (
				db        dal.DB
				colDef    *ingitdb.CollectionDef
				colDefID  string
				recordKey string
				dirPath   string
				def       *ingitdb.Definition
				err       error
			)
			if githubValue != "" {
				spec, parseErr := parseGitHubRepoSpec(githubValue)
				if parseErr != nil {
					return parseErr
				}
				def, collectionID, key, readErr := readRemoteDefinitionForID(ctx, spec, id)
				if readErr != nil {
					return fmt.Errorf("failed to resolve remote definition: %w", readErr)
				}
				cfg := newGitHubConfig(spec, githubToken(cmd))
				db, err = gitHubDBFactory.NewGitHubDBWithDef(cfg, def)
				if err != nil {
					return fmt.Errorf("failed to open github database: %w", err)
				}
				colDefID = collectionID
				recordKey = key
			} else {
				dirPath, resolveErr := resolveDBPath(cmd, homeDir, getWd)
				if resolveErr != nil {
					return resolveErr
				}
				_ = logf
				def, err = readDefinition(dirPath)
				if err != nil {
					return fmt.Errorf("failed to read database definition: %w", err)
				}
				var (
					parseErr error
					key      string
				)
				colDef, key, parseErr = dalgo2ingitdb.CollectionForKey(def, id)
				if parseErr != nil {
					return fmt.Errorf("invalid --id: %w", parseErr)
				}
				colDefID = colDef.ID
				recordKey = key
				db, err = newDB(dirPath, def)
				if err != nil {
					return fmt.Errorf("failed to open database: %w", err)
				}
			}
			key := dal.NewKeyWithID(colDefID, recordKey)
			err = db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
				return tx.Delete(ctx, key)
			})
			if err != nil {
				return err
			}
			if githubValue == "" {
				builder, err := viewBuilderFactory.ViewBuilderForCollection(colDef)
				if err != nil {
					return fmt.Errorf("failed to init view builder for collection %s: %w", colDefID, err)
				}
				if builder != nil {
					if _, buildErr := builder.BuildViews(ctx, dirPath, colDef, def); buildErr != nil {
						return fmt.Errorf("failed to materialize views for collection %s: %w", colDefID, buildErr)
					}
				}
			}
			return nil
		},
	}
}
