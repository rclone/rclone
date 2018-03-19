// Copyright 2017 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// A runner for the cross-language tests.

package firestore

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"path"
	"path/filepath"
	"strings"
	"testing"

	pb "cloud.google.com/go/firestore/genproto"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	ts "github.com/golang/protobuf/ptypes/timestamp"
	"golang.org/x/net/context"
	fspb "google.golang.org/genproto/googleapis/firestore/v1beta1"
)

func TestCrossLanguageTests(t *testing.T) {
	const dir = "testdata"
	fis, err := ioutil.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, fi := range fis {
		if strings.HasSuffix(fi.Name(), ".textproto") {
			// TODO(jba): use  sub-tests.
			runTestFromFile(t, filepath.Join(dir, fi.Name()))
			n++
		}
	}
	t.Logf("ran %d cross-language tests", n)
}

func runTestFromFile(t *testing.T, filename string) {
	bytes, err := ioutil.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	var test pb.Test
	if err := proto.UnmarshalText(string(bytes), &test); err != nil {
		t.Fatalf("unmarshalling %s: %v", filename, err)
	}
	msg := fmt.Sprintf("%s (file %s)", test.Description, filepath.Base(filename))
	runTest(t, msg, &test)
}

func runTest(t *testing.T, msg string, test *pb.Test) {
	check := func(gotErr error, wantErr bool) {
		if wantErr && gotErr == nil {
			t.Errorf("%s: got nil, want error", msg)
		} else if !wantErr && gotErr != nil {
			t.Errorf("%s: %v", msg, gotErr)
		}
	}

	ctx := context.Background()
	c, srv := newMock(t)

	switch tt := test.Test.(type) {
	case *pb.Test_Get:
		srv.addRPC(tt.Get.Request, &fspb.Document{
			CreateTime: &ts.Timestamp{},
			UpdateTime: &ts.Timestamp{},
		})
		ref := docRefFromPath(tt.Get.DocRefPath, c)
		_, err := ref.Get(ctx)
		if err != nil {
			t.Errorf("%s: %v", msg, err)
			return
		}
		// Checking response would just be testing the function converting a Document
		// proto to a DocumentSnapshot, hence uninteresting.

	case *pb.Test_Create:
		srv.addRPC(tt.Create.Request, commitResponseForSet)
		ref := docRefFromPath(tt.Create.DocRefPath, c)
		data := convertData(tt.Create.JsonData)
		_, err := ref.Create(ctx, data)
		check(err, tt.Create.IsError)

	case *pb.Test_Set:
		srv.addRPC(tt.Set.Request, commitResponseForSet)
		ref := docRefFromPath(tt.Set.DocRefPath, c)
		data := convertData(tt.Set.JsonData)
		var opts []SetOption
		if tt.Set.Option != nil {
			opts = []SetOption{convertSetOption(tt.Set.Option)}
		}
		_, err := ref.Set(ctx, data, opts...)
		check(err, tt.Set.IsError)

	case *pb.Test_Update:
		// Ignore Update test because we only support UpdatePaths.
		// Not to worry, every Update test has a corresponding UpdatePaths test.

	case *pb.Test_UpdatePaths:
		srv.addRPC(tt.UpdatePaths.Request, commitResponseForSet)
		ref := docRefFromPath(tt.UpdatePaths.DocRefPath, c)
		preconds := convertPrecondition(t, tt.UpdatePaths.Precondition)
		paths := convertFieldPaths(tt.UpdatePaths.FieldPaths)
		var ups []Update
		for i, path := range paths {
			jsonValue := tt.UpdatePaths.JsonValues[i]
			var val interface{}
			if err := json.Unmarshal([]byte(jsonValue), &val); err != nil {
				t.Fatalf("%s: %q: %v", msg, jsonValue, err)
			}
			ups = append(ups, Update{
				FieldPath: path,
				Value:     convertTestValue(val),
			})
		}
		_, err := ref.Update(ctx, ups, preconds...)
		check(err, tt.UpdatePaths.IsError)

	case *pb.Test_Delete:
		srv.addRPC(tt.Delete.Request, commitResponseForSet)
		ref := docRefFromPath(tt.Delete.DocRefPath, c)
		preconds := convertPrecondition(t, tt.Delete.Precondition)
		_, err := ref.Delete(ctx, preconds...)
		check(err, tt.Delete.IsError)

	default:
		t.Fatalf("unknown test type %T", tt)
	}
}

func docRefFromPath(p string, c *Client) *DocumentRef {
	return &DocumentRef{
		Path:   p,
		ID:     path.Base(p),
		Parent: &CollectionRef{c: c},
	}
}

func convertData(jsonData string) map[string]interface{} {
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(jsonData), &m); err != nil {
		log.Fatal(err)
	}
	return convertTestMap(m)
}

func convertTestMap(m map[string]interface{}) map[string]interface{} {
	for k, v := range m {
		m[k] = convertTestValue(v)
	}
	return m
}

func convertTestValue(v interface{}) interface{} {
	switch v := v.(type) {
	case string:
		switch v {
		case "ServerTimestamp":
			return ServerTimestamp
		case "Delete":
			return Delete
		default:
			return v
		}
	case float64:
		if v == float64(int(v)) {
			return int(v)
		}
		return v
	case []interface{}:
		for i, e := range v {
			v[i] = convertTestValue(e)
		}
		return v
	case map[string]interface{}:
		return convertTestMap(v)
	default:
		return v
	}
}

func convertSetOption(opt *pb.SetOption) SetOption {
	if opt.All {
		return MergeAll
	}
	return Merge(convertFieldPaths(opt.Fields)...)
}

func convertFieldPaths(fps []*pb.FieldPath) []FieldPath {
	var res []FieldPath
	for _, fp := range fps {
		res = append(res, fp.Field)
	}
	return res
}

func convertPrecondition(t *testing.T, fp *fspb.Precondition) []Precondition {
	if fp == nil {
		return nil
	}
	var pc Precondition
	switch fp := fp.ConditionType.(type) {
	case *fspb.Precondition_Exists:
		pc = exists(fp.Exists)
	case *fspb.Precondition_UpdateTime:
		tm, err := ptypes.Timestamp(fp.UpdateTime)
		if err != nil {
			t.Fatal(err)
		}
		pc = LastUpdateTime(tm)
	default:
		t.Fatalf("unknown precondition type %T", fp)
	}
	return []Precondition{pc}
}
