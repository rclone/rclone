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
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"testing"
	"time"
)

type equaller interface {
	equals(equaller) bool
}

func (s *atom) equals(o *atom) bool {
	switch v1 := s.Value.(type) {
	case float64:
		if v2, ok := o.Value.(float64); ok {
			if math.IsNaN(v1) && math.IsNaN(v2) {
				return true
			}
			return v1 == v2
		}
	default:
		return reflect.DeepEqual(s, o)
	}
	return false
}

func (s *value) equals(o *value) bool {
	return reflect.DeepEqual(s, o)
}

func (s Fields) equals(o Fields) bool {
	return reflect.DeepEqual(s, o)
}

func (s opDict) equals(o opDict) bool {
	return reflect.DeepEqual(s, o)
}

func (s *change) equals(o *change) bool {
	return reflect.DeepEqual(s, o)
}

func (s *fieldOp) equals(o *fieldOp) bool {
	return reflect.DeepEqual(s, o)
}

func testDSAtom(t *testing.T, c *atom, e string) {
	var c2 atom
	var err error
	var js []byte

	if js, err = json.Marshal(c); err != nil {
		t.Errorf("%s", err)
	}
	if err = c2.UnmarshalJSON(js); err != nil {
		t.Errorf("%s", err)
	}
	if !c.equals(&c2) {
		t.Errorf("expected %#v type %s got %#v of type %s", c.Value, reflect.TypeOf(c.Value).Name(), c2.Value, reflect.TypeOf(c2.Value).Name())
	}
	c2 = atom{}
	if err = c2.UnmarshalJSON([]byte(e)); err != nil {
		t.Errorf("%s", err)
	}
	if !c.equals(&c2) {
		t.Errorf("expected %#v type %s got %#v of type %s", c.Value, reflect.TypeOf(c.Value).Name(), c2.Value, reflect.TypeOf(c2.Value).Name())
	}
}

func TestDSAtomUnmarshalJSON(t *testing.T) {
	testDSAtom(t, &atom{Value: 32.5}, `32.5`)
	testDSAtom(t, &atom{Value: true}, `true`)
	testDSAtom(t, &atom{Value: int64(42)}, `{"I":"42"}`)
	testDSAtom(t, &atom{Value: math.NaN()}, `{"N":"nan"}`)
	testDSAtom(t, &atom{Value: math.Inf(1)}, `{"N":"+inf"}`)
	testDSAtom(t, &atom{Value: math.Inf(-1)}, `{"N":"-inf"}`)
	testDSAtom(t, &atom{Value: []byte(`random string converted to bytes`)}, `{"B":"cmFuZG9tIHN0cmluZyBjb252ZXJ0ZWQgdG8gYnl0ZXM="}`)

	now := time.Now().Round(time.Millisecond)
	js := fmt.Sprintf(`{"T": "%d"}`, now.UnixNano()/int64(time.Millisecond))
	testDSAtom(t, &atom{Value: now}, js)
}

func testDSChange(t *testing.T, c *change, e string) {
	var c2 change
	var err error
	var js []byte

	if js, err = json.Marshal(c); err != nil {
		t.Errorf("%s", err)
	}
	if err = c2.UnmarshalJSON(js); err != nil {
		t.Errorf("%s", err)
	}
	if !c.equals(&c2) {
		t.Errorf("mismatch: got:\n\t%#v\nexpected:\n\t%#v", c2, *c)
	}
	c2 = change{}
	if err = c2.UnmarshalJSON([]byte(e)); err != nil {
		t.Errorf("%s", err)
	}
	if !c.equals(&c2) {
		t.Errorf("mismatch")
	}
}

func TestDSChangeUnmarshalJSON(t *testing.T) {
	testDSChange(t,
		&change{
			Op:       recordInsert,
			TID:      "dropbox",
			RecordID: "test",
			Data:     Fields{"float": value{values: []interface{}{float64(42)}, isList: false}},
		}, `["I","dropbox","test",{"float":42}]`)
	testDSChange(t,
		&change{
			Op:       recordUpdate,
			TID:      "dropbox",
			RecordID: "test",
			Ops:      opDict{"field": fieldOp{Op: fieldPut, Data: value{values: []interface{}{float64(42)}, isList: false}}},
		}, `["U","dropbox","test",{"field":["P", 42]}]`)
	testDSChange(t,
		&change{
			Op:       recordUpdate,
			TID:      "dropbox",
			RecordID: "test",
			Ops:      opDict{"field": fieldOp{Op: listCreate}},
		}, `["U","dropbox","test",{"field":["LC"]}]`)

	testDSChange(t,
		&change{
			Op:       recordDelete,
			TID:      "dropbox",
			RecordID: "test",
		}, `["D","dropbox","test"]`)
}

func testCheckfieldOp(t *testing.T, fo *fieldOp, e string) {
	var fo2 fieldOp
	var js []byte
	var err error

	if js, err = json.Marshal(fo); err != nil {
		t.Errorf("%s", err)
	}
	if string(js) != e {
		t.Errorf("marshalling error got %s expected %s", string(js), e)
	}
	if err = json.Unmarshal(js, &fo2); err != nil {
		t.Errorf("%s %s", err, string(js))
	}
	if !fo.equals(&fo2) {
		t.Errorf("%#v != %#v\n", fo, fo2)
	}
	fo2 = fieldOp{}
	if err = json.Unmarshal([]byte(e), &fo2); err != nil {
		t.Errorf("%s %s", err, string(js))
	}
	if !fo.equals(&fo2) {
		t.Errorf("%#v != %#v\n", fo, fo2)
	}
}

func TestDSfieldOpMarshalling(t *testing.T) {
	testCheckfieldOp(t, &fieldOp{Op: "P", Data: value{values: []interface{}{"bar"}, isList: false}}, `["P","bar"]`)
	testCheckfieldOp(t, &fieldOp{Op: "P", Data: value{values: []interface{}{"ga", "bu", "zo", "meuh", int64(42), 4.5, true}, isList: true}}, `["P",["ga","bu","zo","meuh",{"I":"42"},4.5,true]]`)
	testCheckfieldOp(t, &fieldOp{Op: "D"}, `["D"]`)
	testCheckfieldOp(t, &fieldOp{Op: "LC"}, `["LC"]`)
	testCheckfieldOp(t, &fieldOp{Op: "LP", Index: 1, Data: value{values: []interface{}{"baz"}}}, `["LP",1,"baz"]`)
	testCheckfieldOp(t, &fieldOp{Op: "LI", Index: 1, Data: value{values: []interface{}{"baz"}}}, `["LI",1,"baz"]`)
	testCheckfieldOp(t, &fieldOp{Op: "LD", Index: 1}, `["LD",1]`)
	testCheckfieldOp(t, &fieldOp{Op: "LM", Index: 1, Index2: 2}, `["LM",1,2]`)
}
