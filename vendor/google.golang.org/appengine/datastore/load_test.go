// Copyright 2016 Google Inc. All Rights Reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

package datastore

import (
	"reflect"
	"testing"

	proto "github.com/golang/protobuf/proto"
	pb "google.golang.org/appengine/internal/datastore"
)

type Simple struct {
	I int64
}

type SimpleWithTag struct {
	I int64 `datastore:"II"`
}

type NestedSimpleWithTag struct {
	A SimpleWithTag `datastore:"AA"`
}

type NestedSliceOfSimple struct {
	A []Simple
}

type SimpleTwoFields struct {
	S  string
	SS string
}

type NestedSimpleAnonymous struct {
	Simple
	X string
}

type NestedSimple struct {
	A Simple
	I int64
}

type NestedSimple1 struct {
	A Simple
	X string
}

type NestedSimple2X struct {
	AA NestedSimple
	A  SimpleTwoFields
	S  string
}

type BDotB struct {
	B string `datastore:"B.B"`
}

type ABDotB struct {
	A BDotB
}

type MultiAnonymous struct {
	Simple
	SimpleTwoFields
	X string
}

var (
	// these values need to be addressable
	testString2 = "two"
	testString3 = "three"
	testInt64   = int64(2)

	fieldNameI         = "I"
	fieldNameX         = "X"
	fieldNameS         = "S"
	fieldNameSS        = "SS"
	fieldNameADotI     = "A.I"
	fieldNameAADotII   = "AA.II"
	fieldNameADotBDotB = "A.B.B"
)

func TestLoadEntityNestedLegacy(t *testing.T) {
	testCases := []struct {
		desc string
		src  *pb.EntityProto
		want interface{}
	}{
		{
			"nested",
			&pb.EntityProto{
				Key: keyToProto("some-app-id", testKey0),
				Property: []*pb.Property{
					&pb.Property{
						Name: &fieldNameX,
						Value: &pb.PropertyValue{
							StringValue: &testString2,
						},
					},
					&pb.Property{
						Name: &fieldNameADotI,
						Value: &pb.PropertyValue{
							Int64Value: &testInt64,
						},
					},
				},
			},
			&NestedSimple1{
				A: Simple{I: testInt64},
				X: testString2,
			},
		},
		{
			"nested with tag",
			&pb.EntityProto{
				Key: keyToProto("some-app-id", testKey0),
				Property: []*pb.Property{
					&pb.Property{
						Name: &fieldNameAADotII,
						Value: &pb.PropertyValue{
							Int64Value: &testInt64,
						},
					},
				},
			},
			&NestedSimpleWithTag{
				A: SimpleWithTag{I: testInt64},
			},
		},
		{
			"nested with anonymous struct field",
			&pb.EntityProto{
				Key: keyToProto("some-app-id", testKey0),
				Property: []*pb.Property{
					&pb.Property{
						Name: &fieldNameX,
						Value: &pb.PropertyValue{
							StringValue: &testString2,
						},
					},
					&pb.Property{
						Name: &fieldNameI,
						Value: &pb.PropertyValue{
							Int64Value: &testInt64,
						},
					},
				},
			},
			&NestedSimpleAnonymous{
				Simple: Simple{I: testInt64},
				X:      testString2,
			},
		},
		{
			"nested with dotted field tag",
			&pb.EntityProto{
				Key: keyToProto("some-app-id", testKey0),
				Property: []*pb.Property{
					&pb.Property{
						Name: &fieldNameADotBDotB,
						Value: &pb.PropertyValue{
							StringValue: &testString2,
						},
					},
				},
			},
			&ABDotB{
				A: BDotB{
					B: testString2,
				},
			},
		},
		{
			"nested with dotted field tag",
			&pb.EntityProto{
				Key: keyToProto("some-app-id", testKey0),
				Property: []*pb.Property{
					&pb.Property{
						Name: &fieldNameI,
						Value: &pb.PropertyValue{
							Int64Value: &testInt64,
						},
					},
					&pb.Property{
						Name: &fieldNameS,
						Value: &pb.PropertyValue{
							StringValue: &testString2,
						},
					},
					&pb.Property{
						Name: &fieldNameSS,
						Value: &pb.PropertyValue{
							StringValue: &testString3,
						},
					},
					&pb.Property{
						Name: &fieldNameX,
						Value: &pb.PropertyValue{
							StringValue: &testString3,
						},
					},
				},
			},
			&MultiAnonymous{
				Simple:          Simple{I: testInt64},
				SimpleTwoFields: SimpleTwoFields{S: "two", SS: "three"},
				X:               "three",
			},
		},
	}

	for _, tc := range testCases {
		dst := reflect.New(reflect.TypeOf(tc.want).Elem()).Interface()
		err := loadEntity(dst, tc.src)
		if err != nil {
			t.Errorf("loadEntity: %s: %v", tc.desc, err)
			continue
		}

		if !reflect.DeepEqual(tc.want, dst) {
			t.Errorf("%s: compare:\ngot:  %#v\nwant: %#v", tc.desc, dst, tc.want)
		}
	}
}

type WithKey struct {
	X string
	I int64
	K *Key `datastore:"__key__"`
}

type NestedWithKey struct {
	N WithKey
	Y string
}

var (
	incompleteKey = newKey("", nil)
	invalidKey    = newKey("s", incompleteKey)

	// these values need to be addressable
	fieldNameA     = "A"
	fieldNameK     = "K"
	fieldNameN     = "N"
	fieldNameY     = "Y"
	fieldNameAA    = "AA"
	fieldNameII    = "II"
	fieldNameBDotB = "B.B"

	entityProtoMeaning = pb.Property_ENTITY_PROTO

	TRUE  = true
	FALSE = false
)

var (
	simpleEntityProto, nestedSimpleEntityProto,
	simpleTwoFieldsEntityProto, simpleWithTagEntityProto,
	bDotBEntityProto, withKeyEntityProto string
)

func init() {
	// simpleEntityProto corresponds to:
	// Simple{I: testInt64}
	simpleEntityProtob, err := proto.Marshal(&pb.EntityProto{
		Key: keyToProto("", incompleteKey),
		Property: []*pb.Property{
			&pb.Property{
				Name: &fieldNameI,
				Value: &pb.PropertyValue{
					Int64Value: &testInt64,
				},
				Multiple: &FALSE,
			},
		},
		EntityGroup: &pb.Path{},
	})
	if err != nil {
		panic(err)
	}
	simpleEntityProto = string(simpleEntityProtob)

	// nestedSimpleEntityProto corresponds to:
	// NestedSimple{
	// 	A: Simple{I: testInt64},
	// 	I: testInt64,
	// }
	nestedSimpleEntityProtob, err := proto.Marshal(&pb.EntityProto{
		Key: keyToProto("", incompleteKey),
		Property: []*pb.Property{
			&pb.Property{
				Name:    &fieldNameA,
				Meaning: &entityProtoMeaning,
				Value: &pb.PropertyValue{
					StringValue: &simpleEntityProto,
				},
				Multiple: &FALSE,
			},
			&pb.Property{
				Name:    &fieldNameI,
				Meaning: &entityProtoMeaning,
				Value: &pb.PropertyValue{
					Int64Value: &testInt64,
				},
				Multiple: &FALSE,
			},
		},
		EntityGroup: &pb.Path{},
	})
	if err != nil {
		panic(err)
	}
	nestedSimpleEntityProto = string(nestedSimpleEntityProtob)

	// simpleTwoFieldsEntityProto corresponds to:
	// SimpleTwoFields{S: testString2, SS: testString3}
	simpleTwoFieldsEntityProtob, err := proto.Marshal(&pb.EntityProto{
		Key: keyToProto("", incompleteKey),
		Property: []*pb.Property{
			&pb.Property{
				Name: &fieldNameS,
				Value: &pb.PropertyValue{
					StringValue: &testString2,
				},
				Multiple: &FALSE,
			},
			&pb.Property{
				Name: &fieldNameSS,
				Value: &pb.PropertyValue{
					StringValue: &testString3,
				},
				Multiple: &FALSE,
			},
		},
		EntityGroup: &pb.Path{},
	})
	if err != nil {
		panic(err)
	}
	simpleTwoFieldsEntityProto = string(simpleTwoFieldsEntityProtob)

	// simpleWithTagEntityProto corresponds to:
	// SimpleWithTag{I: testInt64}
	simpleWithTagEntityProtob, err := proto.Marshal(&pb.EntityProto{
		Key: keyToProto("", incompleteKey),
		Property: []*pb.Property{
			&pb.Property{
				Name: &fieldNameII,
				Value: &pb.PropertyValue{
					Int64Value: &testInt64,
				},
				Multiple: &FALSE,
			},
		},
		EntityGroup: &pb.Path{},
	})
	if err != nil {
		panic(err)
	}
	simpleWithTagEntityProto = string(simpleWithTagEntityProtob)

	// bDotBEntityProto corresponds to:
	// BDotB{
	// 	B: testString2,
	// }
	bDotBEntityProtob, err := proto.Marshal(&pb.EntityProto{
		Key: keyToProto("", incompleteKey),
		Property: []*pb.Property{
			&pb.Property{
				Name: &fieldNameBDotB,
				Value: &pb.PropertyValue{
					StringValue: &testString2,
				},
				Multiple: &FALSE,
			},
		},
		EntityGroup: &pb.Path{},
	})
	if err != nil {
		panic(err)
	}
	bDotBEntityProto = string(bDotBEntityProtob)

	// withKeyEntityProto corresponds to:
	// WithKey{
	// 	X: testString3,
	// 	I: testInt64,
	// 	K: testKey1a,
	// }
	withKeyEntityProtob, err := proto.Marshal(&pb.EntityProto{
		Key: keyToProto("", testKey1a),
		Property: []*pb.Property{
			&pb.Property{
				Name: &fieldNameX,
				Value: &pb.PropertyValue{
					StringValue: &testString3,
				},
				Multiple: &FALSE,
			},
			&pb.Property{
				Name: &fieldNameI,
				Value: &pb.PropertyValue{
					Int64Value: &testInt64,
				},
				Multiple: &FALSE,
			},
		},
		EntityGroup: &pb.Path{},
	})
	if err != nil {
		panic(err)
	}
	withKeyEntityProto = string(withKeyEntityProtob)

}

func TestLoadEntityNested(t *testing.T) {
	testCases := []struct {
		desc string
		src  *pb.EntityProto
		want interface{}
	}{
		{
			"nested basic",
			&pb.EntityProto{
				Key: keyToProto("some-app-id", testKey0),
				Property: []*pb.Property{
					&pb.Property{
						Name:    &fieldNameA,
						Meaning: &entityProtoMeaning,
						Value: &pb.PropertyValue{
							StringValue: &simpleEntityProto,
						},
					},
					&pb.Property{
						Name: &fieldNameI,
						Value: &pb.PropertyValue{
							Int64Value: &testInt64,
						},
					},
				},
			},
			&NestedSimple{
				A: Simple{I: 2},
				I: 2,
			},
		},
		{
			"nested with struct tags",
			&pb.EntityProto{
				Key: keyToProto("some-app-id", testKey0),
				Property: []*pb.Property{
					&pb.Property{
						Name:    &fieldNameAA,
						Meaning: &entityProtoMeaning,
						Value: &pb.PropertyValue{
							StringValue: &simpleWithTagEntityProto,
						},
					},
				},
			},
			&NestedSimpleWithTag{
				A: SimpleWithTag{I: testInt64},
			},
		},
		{
			"nested 2x",
			&pb.EntityProto{
				Key: keyToProto("some-app-id", testKey0),
				Property: []*pb.Property{
					&pb.Property{
						Name:    &fieldNameAA,
						Meaning: &entityProtoMeaning,
						Value: &pb.PropertyValue{
							StringValue: &nestedSimpleEntityProto,
						},
					},
					&pb.Property{
						Name:    &fieldNameA,
						Meaning: &entityProtoMeaning,
						Value: &pb.PropertyValue{
							StringValue: &simpleTwoFieldsEntityProto,
						},
					},
					&pb.Property{
						Name: &fieldNameS,
						Value: &pb.PropertyValue{
							StringValue: &testString3,
						},
					},
				},
			},
			&NestedSimple2X{
				AA: NestedSimple{
					A: Simple{I: testInt64},
					I: testInt64,
				},
				A: SimpleTwoFields{S: testString2, SS: testString3},
				S: testString3,
			},
		},
		{
			"nested anonymous",
			&pb.EntityProto{
				Key: keyToProto("some-app-id", testKey0),
				Property: []*pb.Property{
					&pb.Property{
						Name: &fieldNameI,
						Value: &pb.PropertyValue{
							Int64Value: &testInt64,
						},
					},
					&pb.Property{
						Name: &fieldNameX,
						Value: &pb.PropertyValue{
							StringValue: &testString2,
						},
					},
				},
			},
			&NestedSimpleAnonymous{
				Simple: Simple{I: testInt64},
				X:      testString2,
			},
		},
		{
			"nested simple with slice",
			&pb.EntityProto{
				Key: keyToProto("some-app-id", testKey0),
				Property: []*pb.Property{
					&pb.Property{
						Name:     &fieldNameA,
						Meaning:  &entityProtoMeaning,
						Multiple: &TRUE,
						Value: &pb.PropertyValue{
							StringValue: &simpleEntityProto,
						},
					},
					&pb.Property{
						Name:     &fieldNameA,
						Meaning:  &entityProtoMeaning,
						Multiple: &TRUE,
						Value: &pb.PropertyValue{
							StringValue: &simpleEntityProto,
						},
					},
				},
			},
			&NestedSliceOfSimple{
				A: []Simple{Simple{I: testInt64}, Simple{I: testInt64}},
			},
		},
		{
			"nested with multiple anonymous fields",
			&pb.EntityProto{
				Key: keyToProto("some-app-id", testKey0),
				Property: []*pb.Property{
					&pb.Property{
						Name: &fieldNameI,
						Value: &pb.PropertyValue{
							Int64Value: &testInt64,
						},
					},
					&pb.Property{
						Name: &fieldNameS,
						Value: &pb.PropertyValue{
							StringValue: &testString2,
						},
					},
					&pb.Property{
						Name: &fieldNameSS,
						Value: &pb.PropertyValue{
							StringValue: &testString3,
						},
					},
					&pb.Property{
						Name: &fieldNameX,
						Value: &pb.PropertyValue{
							StringValue: &testString2,
						},
					},
				},
			},
			&MultiAnonymous{
				Simple:          Simple{I: testInt64},
				SimpleTwoFields: SimpleTwoFields{S: testString2, SS: testString3},
				X:               testString2,
			},
		},
		{
			"nested with dotted field tag",
			&pb.EntityProto{
				Key: keyToProto("some-app-id", testKey0),
				Property: []*pb.Property{
					&pb.Property{
						Name:    &fieldNameA,
						Meaning: &entityProtoMeaning,
						Value: &pb.PropertyValue{
							StringValue: &bDotBEntityProto,
						},
					},
				},
			},
			&ABDotB{
				A: BDotB{
					B: testString2,
				},
			},
		},
		{
			"nested entity with key",
			&pb.EntityProto{
				Key: keyToProto("some-app-id", testKey0),
				Property: []*pb.Property{
					&pb.Property{
						Name: &fieldNameY,
						Value: &pb.PropertyValue{
							StringValue: &testString2,
						},
					},
					&pb.Property{
						Name:    &fieldNameN,
						Meaning: &entityProtoMeaning,
						Value: &pb.PropertyValue{
							StringValue: &withKeyEntityProto,
						},
					},
				},
			},
			&NestedWithKey{
				Y: testString2,
				N: WithKey{
					X: testString3,
					I: testInt64,
					K: testKey1a,
				},
			},
		},
	}

	for _, tc := range testCases {
		dst := reflect.New(reflect.TypeOf(tc.want).Elem()).Interface()
		err := loadEntity(dst, tc.src)
		if err != nil {
			t.Errorf("loadEntity: %s: %v", tc.desc, err)
			continue
		}

		if !reflect.DeepEqual(tc.want, dst) {
			t.Errorf("%s: compare:\ngot:  %#v\nwant: %#v", tc.desc, dst, tc.want)
		}
	}
}
