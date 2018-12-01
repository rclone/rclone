package name

// VarCaseSingle version of a name.
//	foo_bar = fooBar
//	admin/widget = adminWidget
//	User = users
func VarCaseSingle(s string) string {
	return New(s).VarCaseSingle().String()
}

// VarCaseSingle version of a name.
//	foo_bar = fooBar
//	admin/widget = adminWidget
//	User = users
func (i Ident) VarCaseSingle() Ident {
	return Ident{i.Group().Singularize().Camelize()}
}

// VarCasePlural version of a name.
//	foo_bar = fooBars
//	admin/widget = adminWidgets
//	User = users
func VarCasePlural(s string) string {
	return New(s).VarCasePlural().String()
}

// VarCasePlural version of a name.
//	foo_bar = fooBars
//	admin/widget = adminWidgets
//	User = users
func (i Ident) VarCasePlural() Ident {
	return Ident{i.Group().Pluralize().Camelize()}
}
