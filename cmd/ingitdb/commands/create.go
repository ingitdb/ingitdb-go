package commands

import (
	"github.com/dal-go/dalgo/dal"
	"github.com/urfave/cli/v3"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb/materializer"
)

// Create returns the create command group.
func Create(
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	newDB func(string, *ingitdb.Definition) (dal.DB, error),
	viewBuilder materializer.ViewBuilder,
	logf func(...any),
) *cli.Command {
	return &cli.Command{
		Name:     "create",
		Aliases:  []string{"c"},
		Usage:    "Create database objects",
		Commands: []*cli.Command{createRecord(homeDir, getWd, readDefinition, newDB, viewBuilder, logf)},
	}
}
