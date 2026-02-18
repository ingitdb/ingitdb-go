package commands

import (
	"context"

	"github.com/urfave/cli/v3"
)

// Import returns the import command.
func Import() *cli.Command {
	return &cli.Command{
		Name:  "import",
		Usage: "Import data from external databases (SQL, GraphQL, etc)",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "connection",
				Usage:    "connection string for the external database",
				Required: true,
			},
			&cli.StringFlag{
				Name:  "path",
				Usage: "path to the inGitDB database directory (default: current directory)",
			},
			&cli.StringFlag{
				Name:  "global-filter",
				Usage: "generic condition applied to all tables (e.g., 'status == \"active\"')",
			},
			&cli.StringSliceFlag{
				Name:  "table",
				Usage: "table to import (can be specified multiple times). Format: table_name or table_name:condition",
			},
		},
		Action: func(_ context.Context, _ *cli.Command) error {
			return cli.Exit("not yet implemented", 1)
		},
	}
}
