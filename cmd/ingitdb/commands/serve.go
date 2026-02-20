package commands

import (
	"context"

	"github.com/dal-go/dalgo/dal"
	"github.com/urfave/cli/v3"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// Serve returns the serve command.
func Serve(
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	newDB func(string, *ingitdb.Definition) (dal.DB, error),
	logf func(...any),
) *cli.Command {
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
				Usage: "enable MCP server over stdio",
			},
			&cli.BoolFlag{
				Name:  "http",
				Usage: "enable HTTP API server",
			},
			&cli.StringFlag{
				Name:  "http-port",
				Value: "8080",
				Usage: "port for HTTP server",
			},
			&cli.StringSliceFlag{
				Name:  "api-domains",
				Usage: "domains that route to the API handler",
			},
			&cli.StringSliceFlag{
				Name:  "mcp-domains",
				Usage: "domains that route to the MCP handler",
			},
			&cli.BoolFlag{
				Name:  "watcher",
				Usage: "enable file watcher",
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			if cmd.Bool("mcp") {
				dirPath, err := resolveDBPath(cmd, homeDir, getWd)
				if err != nil {
					return err
				}
				return serveMCP(ctx, dirPath, readDefinition, newDB, logf)
			}
			if cmd.Bool("http") {
				port := cmd.String("http-port")
				apiDomains := cmd.StringSlice("api-domains")
				mcpDomains := cmd.StringSlice("mcp-domains")
				return serveHTTP(ctx, port, apiDomains, mcpDomains, logf)
			}
			return cli.Exit("no server mode specified; use --mcp, --http, or --watcher", 1)
		},
	}
}
