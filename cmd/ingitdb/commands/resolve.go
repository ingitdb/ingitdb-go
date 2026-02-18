package commands

import (
	"context"

	"github.com/urfave/cli/v3"
)

// Resolve returns the resolve command.
func Resolve() *cli.Command {
	return &cli.Command{
		Name:  "resolve",
		Usage: "Resolve merge conflicts in database files",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "path",
				Usage: "path to the database directory",
			},
			&cli.StringFlag{
				Name:  "file",
				Usage: "specific file to resolve",
			},
		},
		Action: func(_ context.Context, _ *cli.Command) error {
			return cli.Exit("not yet implemented", 1)
		},
	}
}
