// Copyright (C) 2015 Space Monkey, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package monkit

import "reflect"

var f64Type = reflect.TypeOf(float64(0))

type emptyStatSource struct{}

func (emptyStatSource) Stats(cb func(key SeriesKey, field string, val float64)) {}

// StatSourceFromStruct uses the reflect package to implement the Stats call
// across all float64-castable fields of the struct.
func StatSourceFromStruct(key SeriesKey, structData interface{}) StatSource {
	val := deref(reflect.ValueOf(structData))

	typ := val.Type()
	if typ.Kind() != reflect.Struct {
		return emptyStatSource{}
	}

	return StatSourceFunc(func(cb func(key SeriesKey, field string, val float64)) {
		for i := 0; i < typ.NumField(); i++ {
			field := deref(val.Field(i))
			field_type := field.Type()

			if field_type.Kind() == reflect.Struct && field.CanInterface() {
				child_source := StatSourceFromStruct(key, field.Interface())
				child_source.Stats(func(key SeriesKey, field string, val float64) {
					cb(key, typ.Field(i).Name+"."+field, val)
				})

			} else if field_type.ConvertibleTo(f64Type) {
				cb(key, typ.Field(i).Name, field.Convert(f64Type).Float())
			}
		}
	})
}

// if val is a pointer, deref until it isn't
func deref(val reflect.Value) reflect.Value {
	for val.Type().Kind() == reflect.Ptr {
		val = val.Elem()
	}
	return val
}
