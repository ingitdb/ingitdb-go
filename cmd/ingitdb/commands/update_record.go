package commands

import (
	"context"
	"fmt"
	"maps"

	"github.com/dal-go/dalgo/dal"
	"github.com/urfave/cli/v3"
	"gopkg.in/yaml.v3"

	"github.com/ingitdb/ingitdb-cli/pkg/dalgo2ghingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/dalgo2ingitdb"
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
			id := cmd.String("id")
			setStr := cmd.String("set")
			githubValue := cmd.String("github")
			var (
				db        dal.DB
				colDef    *ingitdb.CollectionDef
				recordKey string
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
				_ = logf
				def, readErr := readDefinition(dirPath)
				if readErr != nil {
					return fmt.Errorf("failed to read database definition: %w", readErr)
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
			var patch map[string]any
			if unmarshalErr := yaml.Unmarshal([]byte(setStr), &patch); unmarshalErr != nil {
				return fmt.Errorf("failed to parse --set: %w", unmarshalErr)
			}
			key := dal.NewKeyWithID(colDef.ID, recordKey)
			return db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
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
		},
	}
}
