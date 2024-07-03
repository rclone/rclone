// Package configstruct parses unstructured maps into structures
package configstruct

import (
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/rclone/rclone/fs/config/configmap"
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
	o := reflect.New(typ)
	switch def.(type) {
	case string:
		// return strings unmodified
		newValue = in
	case int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64, uintptr,
		float32, float64:
		// As per Rob Pike's advice in https://github.com/golang/go/issues/43306
		// we only use Sscan for numbers
		var n int
		n, err = fmt.Sscanln(in, o.Interface())
		if err == nil && n != 1 {
			err = errors.New("no items parsed")
		}
		newValue = o.Elem().Interface()
	case bool:
		newValue, err = strconv.ParseBool(in)
	case time.Duration:
		newValue, err = time.ParseDuration(in)
	default:
		// Try using a Set method
		if do, ok := o.Interface().(interface{ Set(s string) error }); ok {
			err = do.Set(in)
		} else {
			err = errors.New("don't know how to parse this type")
		}
		newValue = o.Elem().Interface()
	}
	if err != nil {
		return nil, fmt.Errorf("parsing %q as %T failed: %w", in, def, err)
	}
	return newValue, nil
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
					return fmt.Errorf("couldn't parse config item %q = %q as %T: %w", defaultItem.Name, configValue, defaultItem.Value, err)
				}
			} else {
				newValue = newNewValue
			}
		}
		defStruct.Field(defaultItem.Num).Set(reflect.ValueOf(newValue))
	}
	return nil
}
