package dalgo2ingitdb

import "github.com/dal-go/dalgo/dal"

// parallelRecordsReader manages concurrent reading of records while ensuring thread safety and efficient data processing.
type parallelRecordsReader struct {
	db dal.DB
}
