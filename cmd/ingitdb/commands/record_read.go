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

// ReadRecord returns the read command for fetching a single record.
func ReadRecord(
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	newDB func(string, *ingitdb.Definition) (dal.DB, error),
	logf func(...any),
) *cli.Command {
	return &cli.Command{
		Name:  "read",
		Usage: "Read a single record from a collection",
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
				Name:  "format",
				Usage: "output format: yaml or json",
				Value: "yaml",
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
