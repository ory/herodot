package herodot

func NoLog() Option {
	return func(o *options) {
		o.noLog = true
	}
}

func newOptions(opts []Option) *options {
	o := new(options)
	for _, oo := range opts {
		oo(o)
	}
	return o
}

type Option func(*options)

type options struct {
	noLog bool
}
