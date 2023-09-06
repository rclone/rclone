package generics

func InitNew[T any](p **T) {
	*p = new(T)
}

func SetZero[T any](p *T) {
	*p = ZeroValue[T]()
}
