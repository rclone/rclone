// +build go1.6,codegen

package api

import (
	"testing"
)

func TestSuppressEventStream(t *testing.T) {
	cases := []struct {
		API    *API
		Ops    []string
		Shapes []string
	}{
		{
			API: &API{
				Operations: map[string]*Operation{
					"abc": {
						InputRef: ShapeRef{
							ShapeName: "abcRequest",
						},
						OutputRef: ShapeRef{
							ShapeName: "abcResponse",
						},
					},
					"eventStreamOp": {
						InputRef: ShapeRef{
							ShapeName: "eventStreamOpRequest",
						},
						OutputRef: ShapeRef{
							ShapeName: "eventStreamOpResponse",
						},
					},
				},
				Shapes: map[string]*Shape{
					"abcRequest":           {},
					"abcResponse":          {},
					"eventStreamOpRequest": {},
					"eventStreamOpResponse": {
						MemberRefs: map[string]*ShapeRef{
							"eventStreamShape": {
								ShapeName: "eventStreamShape",
							},
						},
					},
					"eventStreamShape": {
						IsEventStream: true,
					},
				},
			},
			Ops:    []string{"Abc"},
			Shapes: []string{"AbcInput", "AbcOutput"},
		},
	}

	for _, c := range cases {
		c.API.Setup()
		if e, a := c.Ops, c.API.OperationNames(); !stringsEqual(e, a) {
			t.Errorf("expect %v ops, got %v", e, a)
		}

		if e, a := c.Shapes, c.API.ShapeNames(); !stringsEqual(e, a) {
			t.Errorf("expect %v ops, got %v", e, a)
		}
	}
}

func stringsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	for i := 0; i < len(a); i++ {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
