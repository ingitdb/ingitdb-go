package commands

import (
	"context"

	"github.com/urfave/cli/v3"
)

// Query returns the query command.
func Query() *cli.Command {
	return &cli.Command{
		Name:  "query",
		Usage: "Query records from a collection",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "collection",
				Usage:    "collection to query",
				Required: true,
			},
			&cli.StringFlag{
				Name:  "path",
				Usage: "path to the database directory",
			},
			&cli.StringFlag{
				Name:  "format",
				Usage: "output format (json, yaml)",
			},
		},
		Action: func(_ context.Context, _ *cli.Command) error {
			return cli.Exit("not yet implemented", 1)
		},
	}
}
