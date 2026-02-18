package commands

import (
	"context"

	"github.com/urfave/cli/v3"
)

func deleteRecords() *cli.Command {
	return &cli.Command{
		Name:  "records",
		Usage: "Delete individual records from a collection",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "path",
				Usage: "path to the database directory",
			},
			&cli.StringFlag{
				Name:     "collection",
				Usage:    "collection to delete records from",
				Required: true,
			},
			&cli.StringFlag{
				Name:  "in",
				Usage: "regular expression scoping deletion to a sub-path",
			},
			&cli.StringFlag{
				Name:  "filter-name",
				Usage: "pattern to match record names to delete",
			},
		},
		Action: func(_ context.Context, _ *cli.Command) error {
			return cli.Exit("not yet implemented", 1)
		},
	}
}
