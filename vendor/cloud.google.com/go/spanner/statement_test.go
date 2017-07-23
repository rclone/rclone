/*
Copyright 2017 Google Inc. All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package spanner

import (
	"reflect"
	"testing"

	"github.com/golang/protobuf/proto"
	proto3 "github.com/golang/protobuf/ptypes/struct"

	sppb "google.golang.org/genproto/googleapis/spanner/v1"
)

// Test Statement.bindParams.
func TestBindParams(t *testing.T) {
	// Verify Statement.bindParams generates correct values and types.
	st := Statement{
		SQL:    "SELECT id from t_foo WHERE col = @var",
		Params: map[string]interface{}{"var": nil},
	}
	want := &sppb.ExecuteSqlRequest{
		Params: &proto3.Struct{
			Fields: map[string]*proto3.Value{"var": nil},
		},
		ParamTypes: map[string]*sppb.Type{"var": nil},
	}
	for i, test := range []struct {
		val       interface{}
		wantField *proto3.Value
		wantType  *sppb.Type
	}{
		{"abc", stringProto("abc"), stringType()},
		{int64(1), intProto(1), intType()},
		{int(1), intProto(1), intType()},
		{[]int(nil), nullProto(), listType(intType())},
		{[]int{}, listProto(), listType(intType())},
	} {
		st.Params["var"] = test.val
		want.Params.Fields["var"] = test.wantField
		want.ParamTypes["var"] = test.wantType
		got := &sppb.ExecuteSqlRequest{}
		if err := st.bindParams(got); err != nil || !proto.Equal(got, want) {
			t.Errorf("#%d: bind result: \n(%v, %v)\nwant\n(%v, %v)\n", i, got, err, want, nil)
		}
	}

	// Verify type error reporting.
	for _, test := range []struct {
		val     interface{}
		wantErr error
	}{
		{
			struct{}{},
			errBindParam("var", struct{}{}, errEncoderUnsupportedType(struct{}{})),
		},
		{
			nil,
			errBindParam("var", nil, errNilParam),
		},
	} {
		st.Params["var"] = test.val
		var got sppb.ExecuteSqlRequest
		if err := st.bindParams(&got); !reflect.DeepEqual(err, test.wantErr) {
			t.Errorf("value %#v:\ngot:  %v\nwant: %v", test.val, err, test.wantErr)
		}
	}
}

func TestNewStatement(t *testing.T) {
	s := NewStatement("query")
	if got, want := s.SQL, "query"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
