package mapi

import (
	"encoding/json"
	"reflect"
	"strings"

	"github.com/pkg/errors"
)

var ErrNotFound = errors.New("not found")

type Mapi map[string]interface{}

func (mi Mapi) String() string {
	b, _ := mi.MarshalJSON()
	return string(b)
}

func (mi Mapi) Interface() interface{} {
	return mi
}

func (mi Mapi) Pluck(s string) (interface{}, error) {
	keys := strings.Split(s, ":")
	return reduce(keys, mi)
}

func (mi *Mapi) UnmarshalJSON(b []byte) error {
	mm := map[string]interface{}{}
	if err := json.Unmarshal(b, &mm); err != nil {
		return errors.WithStack(err)
	}
	unmarshal(mm)
	(*mi) = Mapi(mm)
	return nil
}

func unmarshal(m map[string]interface{}) {
	for k, v := range m {
		if mv, ok := v.(map[string]interface{}); ok {
			unmarshal(mv)
			m[k] = Mapi(mv)
		}
	}
}

func (mi Mapi) MarshalJSON() ([]byte, error) {
	m := map[string]interface{}{}

	for k, v := range mi {
		rv := reflect.Indirect(reflect.ValueOf(v))
		switch rv.Kind() {
		case reflect.Map:
			mm := Mapi{}
			for _, xk := range rv.MapKeys() {
				mm[xk.String()] = rv.MapIndex(xk).Interface()
			}
			m[k] = mm
		default:
			if _, ok := v.(Mapi); ok {
				continue
			}
			if _, err := json.Marshal(v); err == nil {
				// if it can be marshaled, add it to the map
				m[k] = v
			}
		}
	}
	return json.Marshal(m)
}

func reduce(keys []string, in interface{}) (interface{}, error) {
	if len(keys) == 0 {
		return nil, ErrNotFound
	}

	rv := reflect.Indirect(reflect.ValueOf(in))
	if !rv.IsValid() {
		return nil, ErrNotFound
	}
	if rv.Kind() != reflect.Map {
		return nil, ErrNotFound
	}

	var key reflect.Value
	for _, k := range rv.MapKeys() {
		if k.String() == keys[0] {
			key = k
			break
		}
	}
	if !key.IsValid() {
		return nil, ErrNotFound
	}

	keys = keys[1:]
	iv := rv.MapIndex(key)

	if !iv.IsValid() {
		return nil, ErrNotFound
	}

	if len(keys) == 0 {
		return iv.Interface(), nil
	}

	return reduce(keys, iv.Interface())
}
