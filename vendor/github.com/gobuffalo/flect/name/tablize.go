package name

// Tableize returns an underscore, pluralized string
//	User = users
//	Person = persons
//	Admin/Widget = admin_widgets
func Tableize(s string) string {
	return New(s).Tableize().String()
}

// Tableize returns an underscore, pluralized string
//	User = users
//	Person = persons
//	Admin/Widget = admin_widgets
func (i Ident) Tableize() Ident {
	return Ident{i.Pluralize().Underscore()}
}
