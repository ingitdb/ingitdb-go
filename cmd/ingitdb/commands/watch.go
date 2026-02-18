package commands

import (
	"context"

	"github.com/urfave/cli/v3"
)

// Watch returns the watch command.
func Watch() *cli.Command {
	return &cli.Command{
		Name:  "watch",
		Usage: "Watch database for changes and log events to stdout",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "path",
				Usage: "path to the database directory",
			},
			&cli.StringFlag{
				Name:  "format",
				Usage: "output format: text (default) or json",
			},
		},
		Action: func(_ context.Context, _ *cli.Command) error {
			return cli.Exit("not yet implemented", 1)
		},
	}
}
