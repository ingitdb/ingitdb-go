package dalgo2ingitdb

import (
	"context"
	"net/url"

	"github.com/dal-go/dalgo/dal"
	"github.com/dal-go/dalgo/recordset"
)

var _ dal.DB = (*database)(nil)

func NewDB(localPath string, remoteRepo *url.URL) (dal.DB, error) {
	// TODO: if remoteRepo is nil localPath should point to existing directory
	return &database{
		localPath:  localPath,
		remoteRepo: remoteRepo,
	}, nil
}

type database struct {
	localPath  string
	remoteRepo *url.URL
}

func (db database) ID() string {
	return DatabaseID
}

func (db database) Adapter() dal.Adapter {
	return dal.NewAdapter(DatabaseID, "v0.0.1")
}

// Schema maps ingitdb.Definition to dal.Schema
func (db database) Schema() dal.Schema {
	//TODO implement me
	panic("implement me")
}

// RunReadonlyTransaction pull recent changes from origin (if URL to remote repo is specified)
func (db database) RunReadonlyTransaction(ctx context.Context, f dal.ROTxWorker, options ...dal.TransactionOption) error {
	tx := readonlyTx{db: db}
	return f(ctx, tx)
}

// RunReadwriteTransaction pull recent changes from origin and creates a new local branch
func (db database) RunReadwriteTransaction(ctx context.Context, f dal.RWTxWorker, options ...dal.TransactionOption) error {
	tx := readwriteTx{readonlyTx: readonlyTx{db: db}}
	return f(ctx, tx)
}

func (db database) Get(ctx context.Context, record dal.Record) error {
	//TODO implement me
	panic("implement me")
}

func (db database) Exists(ctx context.Context, key *dal.Key) (bool, error) {
	//TODO implement me
	panic("implement me")
}

func (db database) GetMulti(ctx context.Context, records []dal.Record) error {
	//TODO implement me
	panic("implement me")
}

func (db database) ExecuteQueryToRecordsReader(ctx context.Context, query dal.Query) (dal.RecordsReader, error) {
	//TODO implement me
	panic("implement me")
}

func (db database) ExecuteQueryToRecordsetReader(ctx context.Context, query dal.Query, options ...recordset.Option) (dal.RecordsetReader, error) {
	//TODO implement me
	panic("implement me")
}
