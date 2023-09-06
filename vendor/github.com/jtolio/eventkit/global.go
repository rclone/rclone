package eventkit

var DefaultRegistry = NewRegistry()

func Package() *Scope {
	return DefaultRegistry.Scope(callerPackage(1))
}
