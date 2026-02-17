package dalgo2ingitdb

import (
	"context"

	"github.com/dal-go/dalgo/dal"
	"github.com/dal-go/dalgo/update"
)

var _ dal.ReadwriteTransaction = (*readwriteTx)(nil)

type readwriteTx struct {
	readonlyTx
}

func (r readwriteTx) ID() string {
	//TODO implement me
	panic("implement me")
}

func (r readwriteTx) Set(ctx context.Context, record dal.Record) error {
	//TODO implement me
	panic("implement me")
}

func (r readwriteTx) SetMulti(ctx context.Context, records []dal.Record) error {
	//TODO implement me
	panic("implement me")
}

func (r readwriteTx) Delete(ctx context.Context, key *dal.Key) error {
	//TODO implement me
	panic("implement me")
}

func (r readwriteTx) DeleteMulti(ctx context.Context, keys []*dal.Key) error {
	//TODO implement me
	panic("implement me")
}

func (r readwriteTx) Update(ctx context.Context, key *dal.Key, updates []update.Update, preconditions ...dal.Precondition) error {
	//TODO implement me
	panic("implement me")
}

func (r readwriteTx) UpdateRecord(ctx context.Context, record dal.Record, updates []update.Update, preconditions ...dal.Precondition) error {
	//TODO implement me
	panic("implement me")
}

func (r readwriteTx) UpdateMulti(ctx context.Context, keys []*dal.Key, updates []update.Update, preconditions ...dal.Precondition) error {
	//TODO implement me
	panic("implement me")
}

func (r readwriteTx) Insert(ctx context.Context, record dal.Record, opts ...dal.InsertOption) error {
	//TODO implement me
	panic("implement me")
}

func (r readwriteTx) InsertMulti(ctx context.Context, records []dal.Record, opts ...dal.InsertOption) error {
	//TODO implement me
	panic("implement me")
}
