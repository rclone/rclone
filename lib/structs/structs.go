// Package structs is for manipulating structures with reflection
package structs

import (
	"reflect"
)

// SetFrom sets the public members of a from b
//
// a and b should be pointers to structs
//
// a can be a different type from b
//
// Only the Fields which have the same name and assignable type on a
// and b will be set.
//
// This is useful for copying between almost identical structures that
// are frequently present in auto generated code for cloud storage
// interfaces.
func SetFrom(a, b interface{}) {
	ta := reflect.TypeOf(a).Elem()
	tb := reflect.TypeOf(b).Elem()
	va := reflect.ValueOf(a).Elem()
	vb := reflect.ValueOf(b).Elem()
	for i := 0; i < tb.NumField(); i++ {
		bField := vb.Field(i)
		tbField := tb.Field(i)
		name := tbField.Name
		aField := va.FieldByName(name)
		taField, found := ta.FieldByName(name)
		if found && aField.IsValid() && bField.IsValid() && aField.CanSet() && tbField.Type.AssignableTo(taField.Type) {
			aField.Set(bField)
		}
	}
}

// SetDefaults for a from b
//
// a and b should be pointers to the same kind of struct
//
// This copies the public members only from b to a.  This is useful if
// you can't just use a struct copy because it contains a private
// mutex, e.g. as http.Transport.
func SetDefaults(a, b interface{}) {
	pt := reflect.TypeOf(a)
	t := pt.Elem()
	va := reflect.ValueOf(a).Elem()
	vb := reflect.ValueOf(b).Elem()
	for i := 0; i < t.NumField(); i++ {
		aField := va.Field(i)
		// Set a from b if it is public
		if aField.CanSet() {
			bField := vb.Field(i)
			aField.Set(bField)
		}
	}
}
