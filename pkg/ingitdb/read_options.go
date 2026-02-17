package ingitdb

type ReadOptions struct {
	isValidationRequired bool
}

func (o *ReadOptions) IsValidationRequired() bool {
	return o.isValidationRequired
}

type ReadOption func(*ReadOptions)

func Validate() func(*ReadOptions) {
	return func(o *ReadOptions) {
		o.isValidationRequired = true
	}
}

func NewReadOptions(o ...ReadOption) (opts ReadOptions) {
	for _, opt := range o {
		opt(&opts)
	}
	return
}
