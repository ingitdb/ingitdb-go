package dalgo2ingitdb

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/dal-go/dalgo/dal"
	"github.com/dal-go/dalgo/update"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
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
	_ = ctx
	colDef, recordKey, err := r.resolveCollection(record.Key())
	if err != nil {
		return err
	}
	record.SetError(nil)
	path := resolveRecordPath(colDef, recordKey)
	data := record.Data().(map[string]any)
	return writeRecordToFile(path, colDef.RecordFile.Format, data)
}

func (r readwriteTx) SetMulti(ctx context.Context, records []dal.Record) error {
	//TODO implement me
	panic("implement me")
}

func (r readwriteTx) Delete(ctx context.Context, key *dal.Key) error {
	_ = ctx
	colDef, recordKey, err := r.resolveCollection(key)
	if err != nil {
		return err
	}
	path := resolveRecordPath(colDef, recordKey)
	return deleteRecordFile(path)
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
	_, _ = ctx, opts
	colDef, recordKey, err := r.resolveCollection(record.Key())
	if err != nil {
		return err
	}
	if colDef.RecordFile.RecordType != ingitdb.SingleRecord {
		return fmt.Errorf("not yet implemented for record type %q", colDef.RecordFile.RecordType)
	}
	path := resolveRecordPath(colDef, recordKey)
	_, statErr := os.Stat(path)
	if statErr == nil {
		return fmt.Errorf("record already exists: %s", path)
	}
	if !errors.Is(statErr, os.ErrNotExist) {
		return fmt.Errorf("failed to check file %s: %w", path, statErr)
	}
	record.SetError(nil)
	data := record.Data().(map[string]any)
	return writeRecordToFile(path, colDef.RecordFile.Format, data)
}

func (r readwriteTx) resolveCollection(key *dal.Key) (*ingitdb.CollectionDef, string, error) {
	if r.db.def == nil {
		return nil, "", fmt.Errorf("definition is required: use NewLocalDBWithDef")
	}
	collectionID := key.Collection()
	colDef, ok := r.db.def.Collections[collectionID]
	if !ok {
		return nil, "", fmt.Errorf("collection %q not found in definition", collectionID)
	}
	if colDef.RecordFile == nil {
		return nil, "", fmt.Errorf("collection %q has no record_file definition", collectionID)
	}
	recordKey := fmt.Sprintf("%v", key.ID)
	return colDef, recordKey, nil
}

func (r readwriteTx) InsertMulti(ctx context.Context, records []dal.Record, opts ...dal.InsertOption) error {
	//TODO implement me
	panic("implement me")
}
