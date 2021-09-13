package gqlformatter

// Option configures the Formatter.
type Option interface {
	apply(f *formatter) // apply is unexported, so only the current package can implement this interface.
}

// Options turns a list of Option instances into an Option.
type Options []Option

func (o Options) apply(f *formatter) {
	for _, opt := range o {
		opt.apply(f)
	}
}

// funcOption wraps a function that modifies a Formatter into an implementation
// of the Option interface.
type funcOption struct {
	f func(*formatter)
}

func (fo *funcOption) apply(f *formatter) {
	fo.f(f)
}

func newFuncOption(f func(*formatter)) *funcOption {
	return &funcOption{
		f: f,
	}
}

// WithMinification returns an Option which sets minified output.
func WithMinification() Option {
	return newFuncOption(func(f *formatter) {
		f.colonUnit = ":"
		f.optArgumentSplitted = false
		f.optWhereLogicalOps = false
		f.optCompressed = true
	})
}

// WithIndent returns an Option which sets the tab string to use.
func WithIndent(indent string) Option {
	return newFuncOption(func(f *formatter) {
		f.indentUnit = indent
	})
}
