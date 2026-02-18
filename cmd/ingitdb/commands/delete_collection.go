package commands

import (
	"context"

	"github.com/urfave/cli/v3"
)

func deleteCollection() *cli.Command {
	return &cli.Command{
		Name:  "collection",
		Usage: "Delete a collection and all its records",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "path",
				Usage: "path to the database directory",
			},
			&cli.StringFlag{
				Name:     "collection",
				Usage:    "collection id to delete (e.g. countries/ie/counties)",
				Required: true,
			},
		},
		Action: func(_ context.Context, _ *cli.Command) error {
			return cli.Exit("not yet implemented", 1)
		},
	}
}
