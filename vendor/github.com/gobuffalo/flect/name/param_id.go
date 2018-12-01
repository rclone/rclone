package name

import "strings"

// ParamID returns the string as parameter with _id added
//	user = user_id
//	UserID = user_id
//	admin/widgets = admin_widgets_id
func ParamID(s string) string {
	return New(s).ParamID().String()
}

// ParamID returns the string as parameter with _id added
//	user = user_id
//	UserID = user_id
//	admin/widgets = admin_widgets_id
func (i Ident) ParamID() Ident {
	s := i.Underscore().String()
	s = strings.ToLower(s)
	if strings.HasSuffix(s, "_id") {
		return New(s)
	}
	return New(s + "_id")
}
