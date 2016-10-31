/*
** Copyright (c) 2014 Arnaud Ysmal.  All Rights Reserved.
**
** Redistribution and use in source and binary forms, with or without
** modification, are permitted provided that the following conditions
** are met:
** 1. Redistributions of source code must retain the above copyright
**    notice, this list of conditions and the following disclaimer.
** 2. Redistributions in binary form must reproduce the above copyright
**    notice, this list of conditions and the following disclaimer in the
**    documentation and/or other materials provided with the distribution.
**
** THIS SOFTWARE IS PROVIDED BY THE AUTHOR ``AS IS'' AND ANY EXPRESS
** OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED
** WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
** DISCLAIMED. IN NO EVENT SHALL THE AUTHOR OR CONTRIBUTORS BE LIABLE
** FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL
** DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR
** SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION)
** HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT
** LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY
** OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF
** SUCH DAMAGE.
 */

package dropbox

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

type atom struct {
	Value interface{}
}

func encodeDBase64(b []byte) string {
	return strings.TrimRight(base64.URLEncoding.EncodeToString(b), "=")
}

func decodeDBase64(s string) ([]byte, error) {
	pad := 4 - len(s)%4
	if pad != 4 {
		s += strings.Repeat("=", pad)
	}
	return base64.URLEncoding.DecodeString(s)
}

// MarshalJSON returns the JSON encoding of a.
func (a atom) MarshalJSON() ([]byte, error) {
	switch v := a.Value.(type) {
	case bool, string:
		return json.Marshal(v)
	case float64:
		if math.IsNaN(v) {
			return []byte(`{"N": "nan"}`), nil
		} else if math.IsInf(v, 1) {
			return []byte(`{"N": "+inf"}`), nil
		} else if math.IsInf(v, -1) {
			return []byte(`{"N": "-inf"}`), nil
		}
		return json.Marshal(v)
	case time.Time:
		return []byte(fmt.Sprintf(`{"T": "%d"}`, v.UnixNano()/int64(time.Millisecond))), nil
	case int, int32, int64:
		return []byte(fmt.Sprintf(`{"I": "%d"}`, v)), nil
	case []byte:
		return []byte(fmt.Sprintf(`{"B": "%s"}`, encodeDBase64(v))), nil
	}
	return nil, fmt.Errorf("wrong format")
}

// UnmarshalJSON parses the JSON-encoded data and stores the result in the value pointed to by a.
func (a *atom) UnmarshalJSON(data []byte) error {
	var i interface{}
	var err error

	if err = json.Unmarshal(data, &i); err != nil {
		return err
	}
	switch v := i.(type) {
	case bool, int, int32, int64, float32, float64, string:
		a.Value = v
		return nil
	case map[string]interface{}:
		for key, rval := range v {
			val, ok := rval.(string)
			if !ok {
				return fmt.Errorf("could not parse atom")
			}
			switch key {
			case "I":
				a.Value, err = strconv.ParseInt(val, 10, 64)
				return nil
			case "N":
				switch val {
				case "nan":
					a.Value = math.NaN()
					return nil
				case "+inf":
					a.Value = math.Inf(1)
					return nil
				case "-inf":
					a.Value = math.Inf(-1)
					return nil
				default:
					return fmt.Errorf("unknown special type %s", val)
				}
			case "T":
				t, err := strconv.ParseInt(val, 10, 64)
				if err != nil {
					return fmt.Errorf("could not parse atom")
				}
				a.Value = time.Unix(t/1000, (t%1000)*int64(time.Millisecond))
				return nil

			case "B":
				a.Value, err = decodeDBase64(val)
				return err
			}
		}
	}
	return fmt.Errorf("could not parse atom")
}

// MarshalJSON returns the JSON encoding of v.
func (v value) MarshalJSON() ([]byte, error) {
	if v.isList {
		var a []atom

		a = make([]atom, len(v.values))
		for i := range v.values {
			a[i].Value = v.values[i]
		}
		return json.Marshal(a)
	}
	return json.Marshal(atom{Value: v.values[0]})
}

// UnmarshalJSON parses the JSON-encoded data and stores the result in the value pointed to by v.
func (v *value) UnmarshalJSON(data []byte) error {
	var isArray bool
	var err error
	var a atom
	var as []atom

	for _, d := range data {
		if d == ' ' {
			continue
		}
		if d == '[' {
			isArray = true
		}
		break
	}
	if isArray {
		if err = json.Unmarshal(data, &as); err != nil {
			return err
		}
		v.values = make([]interface{}, len(as))
		for i, at := range as {
			v.values[i] = at.Value
		}
		v.isList = true
		return nil
	}
	if err = json.Unmarshal(data, &a); err != nil {
		return err
	}
	v.values = make([]interface{}, 1)
	v.values[0] = a.Value
	return nil
}

// UnmarshalJSON parses the JSON-encoded data and stores the result in the value pointed to by f.
func (f *fieldOp) UnmarshalJSON(data []byte) error {
	var i []json.RawMessage
	var err error

	if err = json.Unmarshal(data, &i); err != nil {
		return err
	}

	if err = json.Unmarshal(i[0], &f.Op); err != nil {
		return err
	}
	switch f.Op {
	case fieldPut:
		if len(i) != 2 {
			return fmt.Errorf("wrong format")
		}
		return json.Unmarshal(i[1], &f.Data)
	case fieldDelete, listCreate:
		if len(i) != 1 {
			return fmt.Errorf("wrong format")
		}
	case listInsert, listPut:
		if len(i) != 3 {
			return fmt.Errorf("wrong format")
		}
		if err = json.Unmarshal(i[1], &f.Index); err != nil {
			return err
		}
		return json.Unmarshal(i[2], &f.Data)
	case listDelete:
		if len(i) != 2 {
			return fmt.Errorf("wrong format")
		}
		return json.Unmarshal(i[1], &f.Index)
	case listMove:
		if len(i) != 3 {
			return fmt.Errorf("wrong format")
		}
		if err = json.Unmarshal(i[1], &f.Index); err != nil {
			return err
		}
		return json.Unmarshal(i[2], &f.Index2)
	default:
		return fmt.Errorf("wrong format")
	}
	return nil
}

// MarshalJSON returns the JSON encoding of f.
func (f fieldOp) MarshalJSON() ([]byte, error) {
	switch f.Op {
	case fieldPut:
		return json.Marshal([]interface{}{f.Op, f.Data})
	case fieldDelete, listCreate:
		return json.Marshal([]interface{}{f.Op})
	case listInsert, listPut:
		return json.Marshal([]interface{}{f.Op, f.Index, f.Data})
	case listDelete:
		return json.Marshal([]interface{}{f.Op, f.Index})
	case listMove:
		return json.Marshal([]interface{}{f.Op, f.Index, f.Index2})
	}
	return nil, fmt.Errorf("could not marshal Change type")
}

// UnmarshalJSON parses the JSON-encoded data and stores the result in the value pointed to by c.
func (c *change) UnmarshalJSON(data []byte) error {
	var i []json.RawMessage
	var err error

	if err = json.Unmarshal(data, &i); err != nil {
		return err
	}
	if len(i) < 3 {
		return fmt.Errorf("wrong format")
	}

	if err = json.Unmarshal(i[0], &c.Op); err != nil {
		return err
	}
	if err = json.Unmarshal(i[1], &c.TID); err != nil {
		return err
	}
	if err = json.Unmarshal(i[2], &c.RecordID); err != nil {
		return err
	}
	switch c.Op {
	case recordInsert:
		if len(i) != 4 {
			return fmt.Errorf("wrong format")
		}
		if err = json.Unmarshal(i[3], &c.Data); err != nil {
			return err
		}
	case recordUpdate:
		if len(i) != 4 {
			return fmt.Errorf("wrong format")
		}
		if err = json.Unmarshal(i[3], &c.Ops); err != nil {
			return err
		}
	case recordDelete:
		if len(i) != 3 {
			return fmt.Errorf("wrong format")
		}
	default:
		return fmt.Errorf("wrong format")
	}
	return nil
}

// MarshalJSON returns the JSON encoding of c.
func (c change) MarshalJSON() ([]byte, error) {
	switch c.Op {
	case recordInsert:
		return json.Marshal([]interface{}{recordInsert, c.TID, c.RecordID, c.Data})
	case recordUpdate:
		return json.Marshal([]interface{}{recordUpdate, c.TID, c.RecordID, c.Ops})
	case recordDelete:
		return json.Marshal([]interface{}{recordDelete, c.TID, c.RecordID})
	}
	return nil, fmt.Errorf("could not marshal Change type")
}
