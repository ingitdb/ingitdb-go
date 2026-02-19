package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/dal-go/dalgo/dal"
	"github.com/urfave/cli/v3"
	"gopkg.in/yaml.v3"

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
			if cmd.String("github") != "" && cmd.String("path") != "" {
				return fmt.Errorf("--path with --github is not supported yet")
			}
			rctx, err := resolveRecordContext(ctx, cmd, id, homeDir, getWd, readDefinition, newDB)
			if err != nil {
				return err
			}
			key := dal.NewKeyWithID(rctx.colDef.ID, rctx.recordKey)
			data := map[string]any{}
			record := dal.NewRecordWithData(key, data)
			err = rctx.db.RunReadonlyTransaction(ctx, func(ctx context.Context, tx dal.ReadTransaction) error {
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
