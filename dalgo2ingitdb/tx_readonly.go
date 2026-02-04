package dalgo2ingitdb

import (
	"context"

	"github.com/dal-go/dalgo/dal"
	"github.com/dal-go/dalgo/recordset"
)

var _ dal.ReadTransaction = (*readonlyTx)(nil)

type readonlyTx struct {
	db database
}

func (r readonlyTx) Options() dal.TransactionOptions {
	//TODO implement me
	panic("implement me")
}

func (r readonlyTx) Get(ctx context.Context, record dal.Record) error {
	//TODO implement me
	panic("implement me")
}

func (r readonlyTx) Exists(ctx context.Context, key *dal.Key) (bool, error) {
	//TODO implement me
	panic("implement me")
}

func (r readonlyTx) GetMulti(ctx context.Context, records []dal.Record) error {
	//TODO implement me
	panic("implement me")
}

func (r readonlyTx) ExecuteQueryToRecordsReader(ctx context.Context, query dal.Query) (dal.RecordsReader, error) {
	//TODO implement me
	panic("implement me")
}

func (r readonlyTx) ExecuteQueryToRecordsetReader(ctx context.Context, query dal.Query, options ...recordset.Option) (dal.RecordsetReader, error) {
	//TODO implement me
	panic("implement me")
}
