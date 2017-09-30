package ewma

// Copyright (c) 2013 VividCortex, Inc. All rights reserved.
// Please see the LICENSE file for applicable license terms.

import (
	"math"
	"testing"
)

const testMargin = 0.00000001

var samples = [100]float64{
	4599, 5711, 4746, 4621, 5037, 4218, 4925, 4281, 5207, 5203, 5594, 5149,
	4948, 4994, 6056, 4417, 4973, 4714, 4964, 5280, 5074, 4913, 4119, 4522,
	4631, 4341, 4909, 4750, 4663, 5167, 3683, 4964, 5151, 4892, 4171, 5097,
	3546, 4144, 4551, 6557, 4234, 5026, 5220, 4144, 5547, 4747, 4732, 5327,
	5442, 4176, 4907, 3570, 4684, 4161, 5206, 4952, 4317, 4819, 4668, 4603,
	4885, 4645, 4401, 4362, 5035, 3954, 4738, 4545, 5433, 6326, 5927, 4983,
	5364, 4598, 5071, 5231, 5250, 4621, 4269, 3953, 3308, 3623, 5264, 5322,
	5395, 4753, 4936, 5315, 5243, 5060, 4989, 4921, 4480, 3426, 3687, 4220,
	3197, 5139, 6101, 5279,
}

func withinMargin(a, b float64) bool {
	return math.Abs(a-b) <= testMargin
}

func TestSimpleEWMA(t *testing.T) {
	var e SimpleEWMA
	for _, f := range samples {
		e.Add(f)
	}
	if !withinMargin(e.Value(), 4734.500946466118) {
		t.Errorf("e.Value() is %v, wanted %v", e.Value(), 4734.500946466118)
	}
	e.Set(1.0)
	if e.Value() != 1.0 {
		t.Errorf("e.Value() is %d", e.Value())
	}
}

func TestVariableEWMA(t *testing.T) {
	e := NewMovingAverage(30)
	for _, f := range samples {
		e.Add(f)
	}
	if !withinMargin(e.Value(), 4734.500946466118) {
		t.Errorf("e.Value() is %v, wanted %v", e.Value(), 4734.500946466118)
	}
	e.Set(1.0)
	if e.Value() != 1.0 {
		t.Errorf("e.Value() is %d", e.Value())
	}
}

func TestVariableEWMA2(t *testing.T) {
	e := NewMovingAverage(5)
	for _, f := range samples {
		e.Add(f)
	}
	if !withinMargin(e.Value(), 5015.397367486725) {
		t.Errorf("e.Value() is %v, wanted %v", e.Value(), 5015.397367486725)
	}
}

func TestVariableEWMAWarmup(t *testing.T) {
	e := NewMovingAverage(5)
	for i, f := range samples {
		e.Add(f)

		// all values returned during warmup should be 0.0
		if uint8(i) < WARMUP_SAMPLES {
			if e.Value() != 0.0 {
				t.Errorf("e.Value() is %v, expected %v", e.Value(), 0.0)
			}
		}
	}
	e = NewMovingAverage(5)
	e.Set(5)
	e.Add(1)
	if e.Value() >= 5 {
		t.Errorf("e.Value() is %d, expected it to decay towards 0", e.Value())
	}
}

func TestVariableEWMAWarmup2(t *testing.T) {
	e := NewMovingAverage(5)
	testSamples := [12]float64{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 10000, 1}
	for i, f := range testSamples {
		e.Add(f)

		// all values returned during warmup should be 0.0
		if uint8(i) < WARMUP_SAMPLES {
			if e.Value() != 0.0 {
				t.Errorf("e.Value() is %v, expected %v", e.Value(), 0.0)
			}
		}
	}
	if val := e.Value(); val == 1.0 {
		t.Errorf("e.Value() is expected to be greater than %v", 1.0)
	}
}
