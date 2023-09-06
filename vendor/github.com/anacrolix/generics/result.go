package generics

type Result[T any] struct {
	Ok  T
	Err error
}

func ResultFromTuple[T any](t T, err error) Result[T] {
	return Result[T]{
		Ok:  t,
		Err: err,
	}
}

func (r Result[T]) AsTuple() (T, error) {
	return r.Ok, r.Err
}

func (r Result[T]) Unwrap() T {
	if r.Err != nil {
		panic(r.Err)
	}
	return r.Ok
}
