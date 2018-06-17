// Copyright 2011 Google Inc. All Rights Reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

package datastore

import (
	"reflect"
	"sort"
	"testing"
	"time"

	"google.golang.org/appengine"
)

func TestValidPropertyName(t *testing.T) {
	testCases := []struct {
		name string
		want bool
	}{
		// Invalid names.
		{"", false},
		{"'", false},
		{".", false},
		{"..", false},
		{".foo", false},
		{"0", false},
		{"00", false},
		{"X.X.4.X.X", false},
		{"\n", false},
		{"\x00", false},
		{"abc\xffz", false},
		{"foo.", false},
		{"foo..", false},
		{"foo..bar", false},
		{"☃", false},
		{`"`, false},
		// Valid names.
		{"AB", true},
		{"Abc", true},
		{"X.X.X.X.X", true},
		{"_", true},
		{"_0", true},
		{"a", true},
		{"a_B", true},
		{"f00", true},
		{"f0o", true},
		{"fo0", true},
		{"foo", true},
		{"foo.bar", true},
		{"foo.bar.baz", true},
		{"世界", true},
	}
	for _, tc := range testCases {
		got := validPropertyName(tc.name)
		if got != tc.want {
			t.Errorf("%q: got %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestStructCodec(t *testing.T) {
	type oStruct struct {
		O int
	}
	type pStruct struct {
		P int
		Q int
	}
	type rStruct struct {
		R int
		S pStruct
		T oStruct
		oStruct
	}
	type uStruct struct {
		U int
		v int
	}
	type vStruct struct {
		V string `datastore:",noindex"`
	}
	oStructCodec := &structCodec{
		fields: map[string]fieldCodec{
			"O": {path: []int{0}},
		},
		complete: true,
	}
	pStructCodec := &structCodec{
		fields: map[string]fieldCodec{
			"P": {path: []int{0}},
			"Q": {path: []int{1}},
		},
		complete: true,
	}
	rStructCodec := &structCodec{
		fields: map[string]fieldCodec{
			"R": {path: []int{0}},
			"S": {path: []int{1}, structCodec: pStructCodec},
			"T": {path: []int{2}, structCodec: oStructCodec},
			"O": {path: []int{3, 0}},
		},
		complete: true,
	}
	uStructCodec := &structCodec{
		fields: map[string]fieldCodec{
			"U": {path: []int{0}},
		},
		complete: true,
	}
	vStructCodec := &structCodec{
		fields: map[string]fieldCodec{
			"V": {path: []int{0}, noIndex: true},
		},
		complete: true,
	}

	testCases := []struct {
		desc        string
		structValue interface{}
		want        *structCodec
	}{
		{
			"oStruct",
			oStruct{},
			oStructCodec,
		},
		{
			"pStruct",
			pStruct{},
			pStructCodec,
		},
		{
			"rStruct",
			rStruct{},
			rStructCodec,
		},
		{
			"uStruct",
			uStruct{},
			uStructCodec,
		},
		{
			"non-basic fields",
			struct {
				B appengine.BlobKey
				K *Key
				T time.Time
			}{},
			&structCodec{
				fields: map[string]fieldCodec{
					"B": {path: []int{0}},
					"K": {path: []int{1}},
					"T": {path: []int{2}},
				},
				complete: true,
			},
		},
		{
			"struct tags with ignored embed",
			struct {
				A       int `datastore:"a,noindex"`
				B       int `datastore:"b"`
				C       int `datastore:",noindex"`
				D       int `datastore:""`
				E       int
				I       int `datastore:"-"`
				J       int `datastore:",noindex" json:"j"`
				oStruct `datastore:"-"`
			}{},
			&structCodec{
				fields: map[string]fieldCodec{
					"a": {path: []int{0}, noIndex: true},
					"b": {path: []int{1}},
					"C": {path: []int{2}, noIndex: true},
					"D": {path: []int{3}},
					"E": {path: []int{4}},
					"J": {path: []int{6}, noIndex: true},
				},
				complete: true,
			},
		},
		{
			"unexported fields",
			struct {
				A int
				b int
				C int `datastore:"x"`
				d int `datastore:"Y"`
			}{},
			&structCodec{
				fields: map[string]fieldCodec{
					"A": {path: []int{0}},
					"x": {path: []int{2}},
				},
				complete: true,
			},
		},
		{
			"nested and embedded structs",
			struct {
				A   int
				B   int
				CC  oStruct
				DDD rStruct
				oStruct
			}{},
			&structCodec{
				fields: map[string]fieldCodec{
					"A":   {path: []int{0}},
					"B":   {path: []int{1}},
					"CC":  {path: []int{2}, structCodec: oStructCodec},
					"DDD": {path: []int{3}, structCodec: rStructCodec},
					"O":   {path: []int{4, 0}},
				},
				complete: true,
			},
		},
		{
			"struct tags with nested and embedded structs",
			struct {
				A       int     `datastore:"-"`
				B       int     `datastore:"w"`
				C       oStruct `datastore:"xx"`
				D       rStruct `datastore:"y"`
				oStruct `datastore:"z"`
			}{},
			&structCodec{
				fields: map[string]fieldCodec{
					"w":   {path: []int{1}},
					"xx":  {path: []int{2}, structCodec: oStructCodec},
					"y":   {path: []int{3}, structCodec: rStructCodec},
					"z.O": {path: []int{4, 0}},
				},
				complete: true,
			},
		},
		{
			"unexported nested and embedded structs",
			struct {
				a int
				B int
				c uStruct
				D uStruct
				uStruct
			}{},
			&structCodec{
				fields: map[string]fieldCodec{
					"B": {path: []int{1}},
					"D": {path: []int{3}, structCodec: uStructCodec},
					"U": {path: []int{4, 0}},
				},
				complete: true,
			},
		},
		{
			"noindex nested struct",
			struct {
				A oStruct `datastore:",noindex"`
			}{},
			&structCodec{
				fields: map[string]fieldCodec{
					"A": {path: []int{0}, structCodec: oStructCodec, noIndex: true},
				},
				complete: true,
			},
		},
		{
			"noindex slice",
			struct {
				A []string `datastore:",noindex"`
			}{},
			&structCodec{
				fields: map[string]fieldCodec{
					"A": {path: []int{0}, noIndex: true},
				},
				hasSlice: true,
				complete: true,
			},
		},
		{
			"noindex embedded struct slice",
			struct {
				// vStruct has a single field, V, also with noindex.
				A []vStruct `datastore:",noindex"`
			}{},
			&structCodec{
				fields: map[string]fieldCodec{
					"A": {path: []int{0}, structCodec: vStructCodec, noIndex: true},
				},
				hasSlice: true,
				complete: true,
			},
		},
	}

	for _, tc := range testCases {
		got, err := getStructCodec(reflect.TypeOf(tc.structValue))
		if err != nil {
			t.Errorf("%s: getStructCodec: %v", tc.desc, err)
			continue
		}
		// can't reflect.DeepEqual b/c element order in fields map may differ
		if !isEqualStructCodec(got, tc.want) {
			t.Errorf("%s\ngot  %+v\nwant %+v\n", tc.desc, got, tc.want)
		}
	}
}

func isEqualStructCodec(got, want *structCodec) bool {
	if got.complete != want.complete {
		return false
	}
	if got.hasSlice != want.hasSlice {
		return false
	}
	if len(got.fields) != len(want.fields) {
		return false
	}
	for name, wantF := range want.fields {
		gotF := got.fields[name]
		if !reflect.DeepEqual(wantF.path, gotF.path) {
			return false
		}
		if wantF.noIndex != gotF.noIndex {
			return false
		}
		if wantF.structCodec != nil {
			if gotF.structCodec == nil {
				return false
			}
			if !isEqualStructCodec(gotF.structCodec, wantF.structCodec) {
				return false
			}
		}
	}

	return true
}

func TestRepeatedPropertyName(t *testing.T) {
	good := []interface{}{
		struct {
			A int `datastore:"-"`
		}{},
		struct {
			A int `datastore:"b"`
			B int
		}{},
		struct {
			A int
			B int `datastore:"B"`
		}{},
		struct {
			A int `datastore:"B"`
			B int `datastore:"-"`
		}{},
		struct {
			A int `datastore:"-"`
			B int `datastore:"A"`
		}{},
		struct {
			A int `datastore:"B"`
			B int `datastore:"A"`
		}{},
		struct {
			A int `datastore:"B"`
			B int `datastore:"C"`
			C int `datastore:"A"`
		}{},
		struct {
			A int `datastore:"B"`
			B int `datastore:"C"`
			C int `datastore:"D"`
		}{},
	}
	bad := []interface{}{
		struct {
			A int `datastore:"B"`
			B int
		}{},
		struct {
			A int
			B int `datastore:"A"`
		}{},
		struct {
			A int `datastore:"C"`
			B int `datastore:"C"`
		}{},
		struct {
			A int `datastore:"B"`
			B int `datastore:"C"`
			C int `datastore:"B"`
		}{},
	}
	testGetStructCodec(t, good, bad)
}

func TestFlatteningNestedStructs(t *testing.T) {
	type DeepGood struct {
		A struct {
			B []struct {
				C struct {
					D int
				}
			}
		}
	}
	type DeepBad struct {
		A struct {
			B []struct {
				C struct {
					D []int
				}
			}
		}
	}
	type ISay struct {
		Tomato int
	}
	type YouSay struct {
		Tomato int
	}
	type Tweedledee struct {
		Dee int `datastore:"D"`
	}
	type Tweedledum struct {
		Dum int `datastore:"D"`
	}

	good := []interface{}{
		struct {
			X []struct {
				Y string
			}
		}{},
		struct {
			X []struct {
				Y []byte
			}
		}{},
		struct {
			P []int
			X struct {
				Y []int
			}
		}{},
		struct {
			X struct {
				Y []int
			}
			Q []int
		}{},
		struct {
			P []int
			X struct {
				Y []int
			}
			Q []int
		}{},
		struct {
			DeepGood
		}{},
		struct {
			DG DeepGood
		}{},
		struct {
			Foo struct {
				Z int
			} `datastore:"A"`
			Bar struct {
				Z int
			} `datastore:"B"`
		}{},
	}
	bad := []interface{}{
		struct {
			X []struct {
				Y []string
			}
		}{},
		struct {
			X []struct {
				Y []int
			}
		}{},
		struct {
			DeepBad
		}{},
		struct {
			DB DeepBad
		}{},
		struct {
			ISay
			YouSay
		}{},
		struct {
			Tweedledee
			Tweedledum
		}{},
		struct {
			Foo struct {
				Z int
			} `datastore:"A"`
			Bar struct {
				Z int
			} `datastore:"A"`
		}{},
	}
	testGetStructCodec(t, good, bad)
}

func testGetStructCodec(t *testing.T, good []interface{}, bad []interface{}) {
	for _, x := range good {
		if _, err := getStructCodec(reflect.TypeOf(x)); err != nil {
			t.Errorf("type %T: got non-nil error (%s), want nil", x, err)
		}
	}
	for _, x := range bad {
		if _, err := getStructCodec(reflect.TypeOf(x)); err == nil {
			t.Errorf("type %T: got nil error, want non-nil", x)
		}
	}
}

func TestNilKeyIsStored(t *testing.T) {
	x := struct {
		K *Key
		I int
	}{}
	p := PropertyList{}
	// Save x as properties.
	p1, _ := SaveStruct(&x)
	p.Load(p1)
	// Set x's fields to non-zero.
	x.K = &Key{}
	x.I = 2
	// Load x from properties.
	p2, _ := p.Save()
	LoadStruct(&x, p2)
	// Check that x's fields were set to zero.
	if x.K != nil {
		t.Errorf("K field was not zero")
	}
	if x.I != 0 {
		t.Errorf("I field was not zero")
	}
}

func TestSaveStructOmitEmpty(t *testing.T) {
	// Expected props names are sorted alphabetically
	expectedPropNamesForSingles := []string{"EmptyValue", "NonEmptyValue", "OmitEmptyWithValue"}
	expectedPropNamesForSlices := []string{"NonEmptyValue", "NonEmptyValue", "OmitEmptyWithValue", "OmitEmptyWithValue"}

	testOmitted := func(expectedPropNames []string, src interface{}) {
		// t.Helper() - this is available from Go version 1.9, but we also support Go versions 1.6, 1.7, 1.8
		if props, err := SaveStruct(src); err != nil {
			t.Fatal(err)
		} else {
			// Collect names for reporting if diffs from expected and for easier sorting
			actualPropNames := make([]string, len(props))
			for i := range props {
				actualPropNames[i] = props[i].Name
			}
			// Sort actuals for comparing with already sorted expected names
			sort.Sort(sort.StringSlice(actualPropNames))
			if !reflect.DeepEqual(actualPropNames, expectedPropNames) {
				t.Errorf("Expected this properties: %v, got: %v", expectedPropNames, actualPropNames)
			}
		}
	}

	testOmitted(expectedPropNamesForSingles, &struct {
		EmptyValue         int
		NonEmptyValue      int
		OmitEmptyNoValue   int `datastore:",omitempty"`
		OmitEmptyWithValue int `datastore:",omitempty"`
	}{
		NonEmptyValue:      1,
		OmitEmptyWithValue: 2,
	})

	testOmitted(expectedPropNamesForSlices, &struct {
		EmptyValue         []int
		NonEmptyValue      []int
		OmitEmptyNoValue   []int `datastore:",omitempty"`
		OmitEmptyWithValue []int `datastore:",omitempty"`
	}{
		NonEmptyValue:      []int{1, 2},
		OmitEmptyWithValue: []int{3, 4},
	})

	testOmitted(expectedPropNamesForSingles, &struct {
		EmptyValue         bool
		NonEmptyValue      bool
		OmitEmptyNoValue   bool `datastore:",omitempty"`
		OmitEmptyWithValue bool `datastore:",omitempty"`
	}{
		NonEmptyValue:      true,
		OmitEmptyWithValue: true,
	})

	testOmitted(expectedPropNamesForSlices, &struct {
		EmptyValue         []bool
		NonEmptyValue      []bool
		OmitEmptyNoValue   []bool `datastore:",omitempty"`
		OmitEmptyWithValue []bool `datastore:",omitempty"`
	}{
		NonEmptyValue:      []bool{true, true},
		OmitEmptyWithValue: []bool{true, true},
	})

	testOmitted(expectedPropNamesForSingles, &struct {
		EmptyValue         string
		NonEmptyValue      string
		OmitEmptyNoValue   string `datastore:",omitempty"`
		OmitEmptyWithValue string `datastore:",omitempty"`
	}{
		NonEmptyValue:      "s",
		OmitEmptyWithValue: "s",
	})

	testOmitted(expectedPropNamesForSlices, &struct {
		EmptyValue         []string
		NonEmptyValue      []string
		OmitEmptyNoValue   []string `datastore:",omitempty"`
		OmitEmptyWithValue []string `datastore:",omitempty"`
	}{
		NonEmptyValue:      []string{"s1", "s2"},
		OmitEmptyWithValue: []string{"s3", "s4"},
	})

	testOmitted(expectedPropNamesForSingles, &struct {
		EmptyValue         float32
		NonEmptyValue      float32
		OmitEmptyNoValue   float32 `datastore:",omitempty"`
		OmitEmptyWithValue float32 `datastore:",omitempty"`
	}{
		NonEmptyValue:      1.1,
		OmitEmptyWithValue: 1.2,
	})

	testOmitted(expectedPropNamesForSlices, &struct {
		EmptyValue         []float32
		NonEmptyValue      []float32
		OmitEmptyNoValue   []float32 `datastore:",omitempty"`
		OmitEmptyWithValue []float32 `datastore:",omitempty"`
	}{
		NonEmptyValue:      []float32{1.1, 2.2},
		OmitEmptyWithValue: []float32{3.3, 4.4},
	})

	testOmitted(expectedPropNamesForSingles, &struct {
		EmptyValue         time.Time
		NonEmptyValue      time.Time
		OmitEmptyNoValue   time.Time `datastore:",omitempty"`
		OmitEmptyWithValue time.Time `datastore:",omitempty"`
	}{
		NonEmptyValue:      now,
		OmitEmptyWithValue: now,
	})

	testOmitted(expectedPropNamesForSlices, &struct {
		EmptyValue         []time.Time
		NonEmptyValue      []time.Time
		OmitEmptyNoValue   []time.Time `datastore:",omitempty"`
		OmitEmptyWithValue []time.Time `datastore:",omitempty"`
	}{
		NonEmptyValue:      []time.Time{now, now},
		OmitEmptyWithValue: []time.Time{now, now},
	})
}
