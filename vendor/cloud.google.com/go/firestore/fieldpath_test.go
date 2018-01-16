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

package firestore

import (
	"strings"
	"testing"
)

func TestFieldPathValidate(t *testing.T) {
	for _, in := range [][]string{nil, []string{}, []string{"a", "", "b"}} {
		if err := FieldPath(in).validate(); err == nil {
			t.Errorf("%v: want error, got nil", in)
		}
	}
}

func TestFieldPathLess(t *testing.T) {
	for _, test := range []struct {
		in1, in2 string
		want     bool
	}{
		{"a b", "a b", false},
		{"a", "b", true},
		{"b", "a", false},
		{"a", "a b", true},
		{"a b", "a", false},
		{"a b c", "a b d", true},
		{"a b d", "a b c", false},
	} {
		fp1 := FieldPath(strings.Fields(test.in1))
		fp2 := FieldPath(strings.Fields(test.in2))
		got := fp1.less(fp2)
		if got != test.want {
			t.Errorf("%q.less(%q): got %t, want %t", test.in1, test.in2, got, test.want)
		}
	}
}

func TestCheckForPrefix(t *testing.T) {
	for _, test := range []struct {
		in      []string // field paths as space-separated strings
		wantErr bool
	}{
		{in: []string{"a", "b", "c"}, wantErr: false},
		{in: []string{"a b", "b", "c d"}, wantErr: false},
		{in: []string{"a b", "a c", "a d"}, wantErr: false},
		{in: []string{"a b", "b", "b d"}, wantErr: true},
		{in: []string{"a b", "b", "b d"}, wantErr: true},
		{in: []string{"b c d", "c d", "b c"}, wantErr: true},
	} {
		var fps []FieldPath
		for _, s := range test.in {
			fps = append(fps, strings.Fields(s))
		}
		err := checkNoDupOrPrefix(fps)
		if got, want := (err != nil), test.wantErr; got != want {
			t.Errorf("%#v: got '%v', want %t", test.in, err, want)
		}
	}
}

func TestCreateMapFromUpdates(t *testing.T) {
	type M map[string]interface{}

	for _, test := range []struct {
		fpvs []fpv
		want M
	}{
		{
			fpvs: nil,
			want: M{},
		},
		{
			fpvs: []fpv{{[]string{"a"}, 1}, {[]string{"b"}, 2}},
			want: M{"a": 1, "b": 2},
		},
		{
			fpvs: []fpv{{[]string{"a", "b"}, 1}, {[]string{"c"}, 2}},
			want: M{"a": map[string]interface{}{"b": 1}, "c": 2},
		},
		{
			fpvs: []fpv{{[]string{"a", "b"}, 1}, {[]string{"c", "d"}, 2}},
			want: M{
				"a": map[string]interface{}{"b": 1},
				"c": map[string]interface{}{"d": 2},
			},
		},
		{
			fpvs: []fpv{{[]string{"a", "b"}, 1}, {[]string{"a", "c"}, 2}},
			want: M{"a": map[string]interface{}{"b": 1, "c": 2}},
		},
	} {
		gotm := createMapFromUpdates(test.fpvs)
		got := M(gotm)
		if !testEqual(got, test.want) {
			t.Errorf("%v: got %#v, want %#v", test.fpvs, got, test.want)
		}
	}
}

func TestToServiceFieldPath(t *testing.T) {
	for _, test := range []struct {
		in   FieldPath
		want string
	}{
		{[]string{"a"}, "a"},
		{[]string{"a", "b"}, "a.b"},
		{[]string{"a.", "[b*", "c2"}, "`a.`.`[b*`.c2"},
		{[]string{"`a", `b\`}, "`\\`a`.`b\\\\`"},
	} {
		got := test.in.toServiceFieldPath()
		if got != test.want {
			t.Errorf("%v: got %s, want %s", test.in, got, test.want)
		}
	}
}

func TestToServiceFieldPathComponent(t *testing.T) {
	for _, test := range []struct {
		in, want string
	}{
		{"", "``"},
		{"clam_chowder23", "clam_chowder23"},
		{"23skidoo", "`23skidoo`"},
		{"bak`tik", "`bak\\`tik`"},
		{"a\\b", "`a\\\\b`"},
		{"dots.are.confusing", "`dots.are.confusing`"},
	} {
		got := toServiceFieldPathComponent(test.in)
		if got != test.want {
			t.Errorf("%q: got %q, want %q", test.in, got, test.want)
		}
	}
}
