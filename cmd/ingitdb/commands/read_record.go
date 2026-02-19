package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/dal-go/dalgo/dal"
	"github.com/urfave/cli/v3"
	"gopkg.in/yaml.v3"

	"github.com/ingitdb/ingitdb-cli/pkg/dalgo2ingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

func readRecord(
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	newDB func(string, *ingitdb.Definition) (dal.DB, error),
	logf func(...any),
) *cli.Command {
	return &cli.Command{
		Name:  "record",
		Usage: "Read a single record from a collection",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "path",
				Usage: "path to the database directory (default: current directory)",
			},
			&cli.StringFlag{
				Name:  "github",
				Usage: "GitHub source as owner/repo[@branch|tag|commit] (public read-only)",
			},
			&cli.StringFlag{
				Name:  "token",
				Usage: "GitHub personal access token (or set GITHUB_TOKEN env var)",
			},
			&cli.StringFlag{
				Name:     "id",
				Usage:    "record ID in the format collection/path/key (e.g. countries/ie)",
				Required: true,
			},
			&cli.StringFlag{
				Name:  "format",
				Usage: "output format: yaml or json",
				Value: "yaml",
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			_ = logf
			id := cmd.String("id")
			githubValue := cmd.String("github")
			var (
				key *dal.Key
				db  dal.DB
				err error
			)
			if githubValue != "" {
				pathValue := cmd.String("path")
				if pathValue != "" {
					return fmt.Errorf("--path with --github is not supported yet")
				}
				spec, parseErr := parseGitHubRepoSpec(githubValue)
				if parseErr != nil {
					return parseErr
				}
				def, collectionID, recordKey, readErr := readRemoteDefinitionForID(ctx, spec, id)
				if readErr != nil {
					return fmt.Errorf("failed to resolve remote definition: %w", readErr)
				}
				cfg := newGitHubConfig(spec, githubToken(cmd))
				db, err = gitHubDBFactory.NewGitHubDBWithDef(cfg, def)
				if err != nil {
					return fmt.Errorf("failed to open github database: %w", err)
				}
				key = dal.NewKeyWithID(collectionID, recordKey)
			} else {
				dirPath, resolveErr := resolveDBPath(cmd, homeDir, getWd)
				if resolveErr != nil {
					return resolveErr
				}
				def, readErr := readDefinition(dirPath)
				if readErr != nil {
					return fmt.Errorf("failed to read database definition: %w", readErr)
				}
				colDef, recordKey, parseErr := dalgo2ingitdb.CollectionForKey(def, id)
				if parseErr != nil {
					return fmt.Errorf("invalid --id: %w", parseErr)
				}
				db, err = newDB(dirPath, def)
				if err != nil {
					return fmt.Errorf("failed to open database: %w", err)
				}
				key = dal.NewKeyWithID(colDef.ID, recordKey)
			}
			data := map[string]any{}
			record := dal.NewRecordWithData(key, data)
			err = db.RunReadonlyTransaction(ctx, func(ctx context.Context, tx dal.ReadTransaction) error {
				return tx.Get(ctx, record)
			})
			if err != nil {
				return err
			}
			if !record.Exists() {
				return fmt.Errorf("record not found: %s", id)
			}
			format := cmd.String("format")
			switch format {
			case "yaml", "yml":
				out, marshalErr := yaml.Marshal(data)
				if marshalErr != nil {
					return fmt.Errorf("failed to marshal output as YAML: %w", marshalErr)
				}
				_, _ = os.Stdout.Write(out)
			case "json":
				out, marshalErr := json.MarshalIndent(data, "", "  ")
				if marshalErr != nil {
					return fmt.Errorf("failed to marshal output as JSON: %w", marshalErr)
				}
				_, _ = fmt.Fprintf(os.Stdout, "%s\n", out)
			default:
				return fmt.Errorf("unknown format %q, use yaml or json", format)
			}
			return nil
		},
	}
}
