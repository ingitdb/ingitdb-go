package commands

import (
	"context"

	"github.com/urfave/cli/v3"
)

// Materialize returns the materialize command.
func Materialize() *cli.Command {
	return &cli.Command{
		Name:  "materialize",
		Usage: "Materialize views in the database",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "path",
				Usage: "path to the database directory",
			},
			&cli.StringFlag{
				Name:  "views",
				Usage: "comma-separated list of views to materialize",
			},
		},
		Action: func(_ context.Context, _ *cli.Command) error {
			return cli.Exit("not yet implemented", 1)
		},
	}
}
