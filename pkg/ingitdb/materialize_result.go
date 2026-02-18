package ingitdb

// MaterializeResult summarises the outcome of a materialisation run.
type MaterializeResult struct {
	FilesWritten   int
	FilesUnchanged int
	Errors         []error
}
