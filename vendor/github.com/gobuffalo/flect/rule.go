package flect

type ruleFn func(string) string

type rule struct {
	suffix string
	fn     ruleFn
}

func noop(s string) string { return s }
