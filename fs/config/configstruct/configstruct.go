// Package configstruct parses unstructured maps into structures
package configstruct

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"

	"github.com/ncw/rclone/fs/config/configmap"
	"github.com/pkg/errors"
)

var matchUpper = regexp.MustCompile("([A-Z]+)")

// camelToSnake converts CamelCase to snake_case
func camelToSnake(in string) string {
	out := matchUpper.ReplaceAllString(in, "_$1")
	out = strings.ToLower(out)
	out = strings.Trim(out, "_")
	return out
}

// StringToInterface turns in into an interface{} the same type as def
func StringToInterface(def interface{}, in string) (newValue interface{}, err error) {
	typ := reflect.TypeOf(def)
	switch typ.Kind() {
	case reflect.String:
		// Pass strings unmodified
		return in, nil
	}
	// Otherwise parse with Sscanln
	//
	// This means any types we use here must implement fmt.Scanner
	o := reflect.New(typ)
	n, err := fmt.Sscanln(in, o.Interface())
	if err != nil {
		return newValue, errors.Wrapf(err, "parsing %q as %T failed", in, def)
	}
	if n != 1 {
		return newValue, errors.New("no items parsed")
	}
	return o.Elem().Interface(), nil
}

// Item describes a single entry in the options structure
type Item struct {
	Name  string // snake_case
	Field string // CamelCase
	Num   int    // number of the field in the struct
	Value interface{}
}

// Items parses the opt struct and returns a slice of Item objects.
//
// opt must be a pointer to a struct.  The struct should have entirely
// public fields.
//
// The config_name is looked up in a struct tag called "config" or if
// not found is the field name converted from CamelCase to snake_case.
func Items(opt interface{}) (items []Item, err error) {
	def := reflect.ValueOf(opt)
	if def.Kind() != reflect.Ptr {
		return nil, errors.New("argument must be a pointer")
	}
	def = def.Elem() // indirect the pointer
	if def.Kind() != reflect.Struct {
		return nil, errors.New("argument must be a pointer to a struct")
	}
	defType := def.Type()
	for i := 0; i < def.NumField(); i++ {
		field := defType.Field(i)
		fieldName := field.Name
		configName, ok := field.Tag.Lookup("config")
		if !ok {
			configName = camelToSnake(fieldName)
		}
		defaultItem := Item{
			Name:  configName,
			Field: fieldName,
			Num:   i,
			Value: def.Field(i).Interface(),
		}
		items = append(items, defaultItem)
	}
	return items, nil
}

// Set interprets the field names in defaults and looks up config
// values in the config passed in.  Any values found in config will be
// set in the opt structure.
//
// opt must be a pointer to a struct.  The struct should have entirely
// public fields.  The field names are converted from CamelCase to
// snake_case and looked up in the config supplied or a
// `config:"field_name"` is looked up.
//
// If items are found then they are converted from string to native
// types and set in opt.
//
// All the field types in the struct must implement fmt.Scanner.
func Set(config configmap.Getter, opt interface{}) (err error) {
	defaultItems, err := Items(opt)
	if err != nil {
		return err
	}
	defStruct := reflect.ValueOf(opt).Elem()
	for _, defaultItem := range defaultItems {
		newValue := defaultItem.Value
		if configValue, ok := config.Get(defaultItem.Name); ok {
			var newNewValue interface{}
			newNewValue, err = StringToInterface(newValue, configValue)
			if err != nil {
				// Mask errors if setting an empty string as
				// it isn't valid for all types.  This makes
				// empty string be the equivalent of unset.
				if configValue != "" {
					return errors.Wrapf(err, "couldn't parse config item %q = %q as %T", defaultItem.Name, configValue, defaultItem.Value)
				}
			} else {
				newValue = newNewValue
			}
		}
		defStruct.Field(defaultItem.Num).Set(reflect.ValueOf(newValue))
	}
	return nil
}
