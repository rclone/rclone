// Package configstruct parses unstructured maps into structures
package configstruct

import (
	"encoding/csv"
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
//
// This supports a subset of builtin types, string, integer types,
// bool, time.Duration and []string.
//
// Builtin types are expected to be encoding as their natural
// stringificatons as produced by fmt.Sprint except for []string which
// is expected to be encoded a a CSV with empty array encoded as "".
//
// Any other types are expected to be encoded by their String()
// methods and decoded by their `Set(s string) error` methods.
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
	case []string:
		// CSV decode arrays of strings - ideally we would use
		// fs.CommaSepList here but we can't as it would cause
		// a circular import.
		if len(in) == 0 {
			newValue = []string{}
		} else {
			r := csv.NewReader(strings.NewReader(in))
			newValue, err = r.Read()
			switch _err := err.(type) {
			case *csv.ParseError:
				err = _err.Err // remove line numbers from the error message
			}
		}
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
	Name  string            // snake_case
	Field string            // CamelCase
	Set   func(interface{}) // set this field
	Value interface{}
}

// Items parses the opt struct and returns a slice of Item objects.
//
// opt must be a pointer to a struct.  The struct should have entirely
// public fields.
//
// The config_name is looked up in a struct tag called "config" or if
// not found is the field name converted from CamelCase to snake_case.
//
// Nested structs are looked up too. If the parent struct has a struct
// tag, this will be used as a prefix for the values in the sub
// struct, otherwise they will be embedded as they are.
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
		field := def.Field(i)
		fieldType := defType.Field(i)
		fieldName := fieldType.Name
		configName, hasTag := fieldType.Tag.Lookup("config")
		if hasTag && configName == "-" {
			// Skip items with config:"-"
			continue
		}
		if !hasTag {
			configName = camelToSnake(fieldName)
		}
		valuePtr := field.Addr().Interface()                   // pointer to the value as an interface
		_, canSet := valuePtr.(interface{ Set(string) error }) // can we set this with the Option Set protocol
		// If we have a nested struct that isn't a config item then recurse
		if fieldType.Type.Kind() == reflect.Struct && !canSet {
			newItems, err := Items(valuePtr)
			if err != nil {
				return nil, fmt.Errorf("error parsing field %q: %w", fieldName, err)
			}
			for _, newItem := range newItems {
				if hasTag {
					newItem.Name = configName + "_" + newItem.Name
				}
				items = append(items, newItem)
			}
		} else {
			defaultItem := Item{
				Name:  configName,
				Field: fieldName,
				Set: func(newValue interface{}) {
					field.Set(reflect.ValueOf(newValue))
				},
				Value: field.Interface(),
			}
			items = append(items, defaultItem)
		}
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
		defaultItem.Set(newValue)
	}
	return nil
}
