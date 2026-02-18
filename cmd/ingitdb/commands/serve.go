package commands

import (
	"context"

	"github.com/urfave/cli/v3"
)

// Serve returns the serve command.
func Serve() *cli.Command {
	return &cli.Command{
		Name:  "serve",
		Usage: "Start one or more servers (MCP, HTTP API, watcher)",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "path",
				Usage: "path to the database directory",
			},
			&cli.BoolFlag{
				Name:  "mcp",
				Usage: "enable MCP server",
			},
			&cli.BoolFlag{
				Name:  "http",
				Usage: "enable HTTP API server",
			},
			&cli.BoolFlag{
				Name:  "watcher",
				Usage: "enable file watcher",
			},
		},
		Action: func(_ context.Context, _ *cli.Command) error {
			return cli.Exit("not yet implemented", 1)
		},
	}
}
