package commands

import (
	"context"

	"github.com/urfave/cli/v3"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// testDef returns a Definition with a single SingleRecord YAML collection at dirPath.
func testDef(dirPath string) *ingitdb.Definition {
	return &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"test.items": {
				ID:      "test.items",
				DirPath: dirPath,
				RecordFile: &ingitdb.RecordFileDef{
					Name:       "{key}.yaml",
					Format:     "yaml",
					RecordType: ingitdb.SingleRecord,
				},
				Columns: map[string]*ingitdb.ColumnDef{
					"name": {Type: ingitdb.ColumnTypeString},
				},
			},
		},
	}
}

// runCLICommand wraps cmd in an app and runs it with the given subcommand arguments.
func runCLICommand(cmd *cli.Command, args ...string) error {
	app := &cli.Command{
		Commands: []*cli.Command{cmd},
	}
	argv := append([]string{"app", cmd.Name}, args...)
	return app.Run(context.Background(), argv)
}
