package commands

import (
	"context"

	"github.com/urfave/cli/v3"
)

func deleteView() *cli.Command {
	return &cli.Command{
		Name:  "view",
		Usage: "Delete a view definition and its materialised files",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "path",
				Usage: "path to the database directory",
			},
			&cli.StringFlag{
				Name:     "view",
				Usage:    "view id to delete",
				Required: true,
			},
		},
		Action: func(_ context.Context, _ *cli.Command) error {
			return cli.Exit("not yet implemented", 1)
		},
	}
}
