package commands

import (
	"context"
	"fmt"

	"github.com/dal-go/dalgo/dal"
	"github.com/urfave/cli/v3"
	"gopkg.in/yaml.v3"

	"github.com/ingitdb/ingitdb-cli/pkg/dalgo2ghingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/dalgo2ingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb/materializer"
)

func createRecord(
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	newDB func(string, *ingitdb.Definition) (dal.DB, error),
	viewBuilder materializer.ViewBuilder,
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
			githubValue := cmd.String("github")
			var (
				db        dal.DB
				colDef    *ingitdb.CollectionDef
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
				db, err = dalgo2ghingitdb.NewGitHubDBWithDef(cfg, def)
				if err != nil {
					return fmt.Errorf("failed to open github database: %w", err)
				}
				colDef = def.Collections[collectionID]
				if colDef == nil {
					return fmt.Errorf("collection not found: %s", collectionID)
				}
				recordKey = key
			} else {
				dirPath, resolveErr := resolveDBPath(cmd, homeDir, getWd)
				if resolveErr != nil {
					return resolveErr
				}
				logf("inGitDB db path: ", dirPath)
				def, err = readDefinition(dirPath)
				if err != nil {
					return fmt.Errorf("failed to read database definition: %w", err)
				}
				var parseErr error
				colDef, recordKey, parseErr = dalgo2ingitdb.CollectionForKey(def, id)
				if parseErr != nil {
					return fmt.Errorf("invalid --id: %w", parseErr)
				}
				db, err = newDB(dirPath, def)
				if err != nil {
					return fmt.Errorf("failed to open database: %w", err)
				}
			}
			var data map[string]any
			if unmarshalErr := yaml.Unmarshal([]byte(dataStr), &data); unmarshalErr != nil {
				return fmt.Errorf("failed to parse --data: %w", unmarshalErr)
			}
			key := dal.NewKeyWithID(colDef.ID, recordKey)
			record := dal.NewRecordWithData(key, data)
			err = db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
				return tx.Insert(ctx, record)
			})
			if err != nil {
				return err
			}
			builder := viewBuilder
			if builder == nil && githubValue == "" {
				builder, err = viewBuilderForCollection(colDef)
				if err != nil {
					return fmt.Errorf("failed to init view builder for collection %s: %w", colDef.ID, err)
				}
			}
			if builder != nil && githubValue == "" {
				if _, buildErr := builder.BuildViews(ctx, dirPath, colDef, def); buildErr != nil {
					return fmt.Errorf("failed to materialize views for collection %s: %w", colDef.ID, buildErr)
				}
			}
			return nil
		},
	}
}

func viewBuilderForCollection(colDef *ingitdb.CollectionDef) (materializer.ViewBuilder, error) {
	if colDef == nil {
		return nil, nil
	}
	reader := materializer.FileViewDefReader{}
	views, err := reader.ReadViewDefs(colDef.DirPath)
	if err != nil {
		return nil, err
	}
	if len(views) == 0 {
		return nil, nil
	}
	// Use the filesystem reader for template-based views like README builders.
	return materializer.NewViewBuilder(materializer.NewFileRecordsReader()), nil
}
