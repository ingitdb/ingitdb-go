package commands

import (
	"context"

	"github.com/urfave/cli/v3"
)

// Setup returns the setup command.
func Setup() *cli.Command {
	return &cli.Command{
		Name:  "setup",
		Usage: "Set up a new inGitDB database",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "path",
				Usage: "path to the database directory",
			},
		},
		Action: func(_ context.Context, _ *cli.Command) error {
			return cli.Exit("not yet implemented", 1)
		},
	}
}
