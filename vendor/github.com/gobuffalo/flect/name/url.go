package name

func (n Ident) URL() Ident {
	return Ident{n.File().Pluralize()}
}
