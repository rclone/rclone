package convert

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestStringValue(t *testing.T) {
	s := "string"
	assert.Equal(t, s, StringValue(String(s)))

	assert.Equal(t, "", StringValue(nil))
}

var testCasesStringSlice = [][]string{
	{"a", "b", "c", "d", "e"},
	{"a", "b", "", "", "e"},
}

func TestStringSlice(t *testing.T) {
	for idx, in := range testCasesStringSlice {
		if in == nil {
			continue
		}
		out := StringSlice(in)
		assert.Len(t, out, len(in), "Unexpected len at idx %d", idx)
		for i := range out {
			assert.Equal(t, in[i], *(out[i]), "Unexpected value at idx %d", idx)
		}

		out2 := StringValueSlice(out)
		assert.Len(t, out2, len(in), "Unexpected len at idx %d", idx)
		assert.Equal(t, in, out2, "Unexpected value at idx %d", idx)
	}
}

var testCasesStringValueSlice = [][]*string{
	{String("a"), String("b"), nil, String("c")},
}

func TestStringValueSlice(t *testing.T) {
	for idx, in := range testCasesStringValueSlice {
		if in == nil {
			continue
		}
		out := StringValueSlice(in)
		assert.Len(t, out, len(in), "Unexpected len at idx %d", idx)
		for i := range out {
			if in[i] == nil {
				assert.Empty(t, out[i], "Unexpected value at idx %d", idx)
			} else {
				assert.Equal(t, *(in[i]), out[i], "Unexpected value at idx %d", idx)
			}
		}

		out2 := StringSlice(out)
		assert.Len(t, out2, len(in), "Unexpected len at idx %d", idx)
		for i := range out2 {
			if in[i] == nil {
				assert.Empty(t, *(out2[i]), "Unexpected value at idx %d", idx)
			} else {
				assert.Equal(t, in[i], out2[i], "Unexpected value at idx %d", idx)
			}
		}
	}
}

var testCasesStringMap = []map[string]string{
	{"a": "1", "b": "2", "c": "3"},
}

func TestStringMap(t *testing.T) {
	for idx, in := range testCasesStringMap {
		if in == nil {
			continue
		}
		out := StringMap(in)
		assert.Len(t, out, len(in), "Unexpected len at idx %d", idx)
		for i := range out {
			assert.Equal(t, in[i], *(out[i]), "Unexpected value at idx %d", idx)
		}

		out2 := StringValueMap(out)
		assert.Len(t, out2, len(in), "Unexpected len at idx %d", idx)
		assert.Equal(t, in, out2, "Unexpected value at idx %d", idx)
	}
}

func TestBoolValue(t *testing.T) {
	b := true
	assert.Equal(t, b, BoolValue(Bool(b)))

	assert.Equal(t, false, BoolValue(nil))
}

var testCasesBoolSlice = [][]bool{
	{true, true, false, false},
}

func TestBoolSlice(t *testing.T) {
	for idx, in := range testCasesBoolSlice {
		if in == nil {
			continue
		}
		out := BoolSlice(in)
		assert.Len(t, out, len(in), "Unexpected len at idx %d", idx)
		for i := range out {
			assert.Equal(t, in[i], *(out[i]), "Unexpected value at idx %d", idx)
		}

		out2 := BoolValueSlice(out)
		assert.Len(t, out2, len(in), "Unexpected len at idx %d", idx)
		assert.Equal(t, in, out2, "Unexpected value at idx %d", idx)
	}
}

var testCasesBoolValueSlice = [][]*bool{}

func TestBoolValueSlice(t *testing.T) {
	for idx, in := range testCasesBoolValueSlice {
		if in == nil {
			continue
		}
		out := BoolValueSlice(in)
		assert.Len(t, out, len(in), "Unexpected len at idx %d", idx)
		for i := range out {
			if in[i] == nil {
				assert.Empty(t, out[i], "Unexpected value at idx %d", idx)
			} else {
				assert.Equal(t, *(in[i]), out[i], "Unexpected value at idx %d", idx)
			}
		}

		out2 := BoolSlice(out)
		assert.Len(t, out2, len(in), "Unexpected len at idx %d", idx)
		for i := range out2 {
			if in[i] == nil {
				assert.Empty(t, *(out2[i]), "Unexpected value at idx %d", idx)
			} else {
				assert.Equal(t, in[i], out2[i], "Unexpected value at idx %d", idx)
			}
		}
	}
}

var testCasesBoolMap = []map[string]bool{
	{"a": true, "b": false, "c": true},
}

func TestBoolMap(t *testing.T) {
	for idx, in := range testCasesBoolMap {
		if in == nil {
			continue
		}
		out := BoolMap(in)
		assert.Len(t, out, len(in), "Unexpected len at idx %d", idx)
		for i := range out {
			assert.Equal(t, in[i], *(out[i]), "Unexpected value at idx %d", idx)
		}

		out2 := BoolValueMap(out)
		assert.Len(t, out2, len(in), "Unexpected len at idx %d", idx)
		assert.Equal(t, in, out2, "Unexpected value at idx %d", idx)
	}
}

func TestIntValue(t *testing.T) {
	i := 1024
	assert.Equal(t, i, IntValue(Int(i)))

	assert.Equal(t, 0, IntValue(nil))
}

var testCasesIntSlice = [][]int{
	{1, 2, 3, 4},
}

func TestIntSlice(t *testing.T) {
	for idx, in := range testCasesIntSlice {
		if in == nil {
			continue
		}
		out := IntSlice(in)
		assert.Len(t, out, len(in), "Unexpected len at idx %d", idx)
		for i := range out {
			assert.Equal(t, in[i], *(out[i]), "Unexpected value at idx %d", idx)
		}

		out2 := IntValueSlice(out)
		assert.Len(t, out2, len(in), "Unexpected len at idx %d", idx)
		assert.Equal(t, in, out2, "Unexpected value at idx %d", idx)
	}
}

var testCasesIntValueSlice = [][]*int{}

func TestIntValueSlice(t *testing.T) {
	for idx, in := range testCasesIntValueSlice {
		if in == nil {
			continue
		}
		out := IntValueSlice(in)
		assert.Len(t, out, len(in), "Unexpected len at idx %d", idx)
		for i := range out {
			if in[i] == nil {
				assert.Empty(t, out[i], "Unexpected value at idx %d", idx)
			} else {
				assert.Equal(t, *(in[i]), out[i], "Unexpected value at idx %d", idx)
			}
		}

		out2 := IntSlice(out)
		assert.Len(t, out2, len(in), "Unexpected len at idx %d", idx)
		for i := range out2 {
			if in[i] == nil {
				assert.Empty(t, *(out2[i]), "Unexpected value at idx %d", idx)
			} else {
				assert.Equal(t, in[i], out2[i], "Unexpected value at idx %d", idx)
			}
		}
	}
}

var testCasesIntMap = []map[string]int{
	{"a": 3, "b": 2, "c": 1},
}

func TestIntMap(t *testing.T) {
	for idx, in := range testCasesIntMap {
		if in == nil {
			continue
		}
		out := IntMap(in)
		assert.Len(t, out, len(in), "Unexpected len at idx %d", idx)
		for i := range out {
			assert.Equal(t, in[i], *(out[i]), "Unexpected value at idx %d", idx)
		}

		out2 := IntValueMap(out)
		assert.Len(t, out2, len(in), "Unexpected len at idx %d", idx)
		assert.Equal(t, in, out2, "Unexpected value at idx %d", idx)
	}
}

func TestInt32Value(t *testing.T) {
	i := int32(1024)
	assert.Equal(t, i, Int32Value(Int32(i)))

	assert.Equal(t, int32(0), Int32Value(nil))
}

var testCasesInt32Slice = [][]int32{
	{1, 2, 3, 4},
}

func TestInt32Slice(t *testing.T) {
	for idx, in := range testCasesInt32Slice {
		if in == nil {
			continue
		}
		out := Int32Slice(in)
		assert.Len(t, out, len(in), "Unexpected len at idx %d", idx)
		for i := range out {
			assert.Equal(t, in[i], *(out[i]), "Unexpected value at idx %d", idx)
		}

		out2 := Int32ValueSlice(out)
		assert.Len(t, out2, len(in), "Unexpected len at idx %d", idx)
		assert.Equal(t, in, out2, "Unexpected value at idx %d", idx)
	}
}

var testCasesInt32ValueSlice = [][]*int32{}

func TestInt32ValueSlice(t *testing.T) {
	for idx, in := range testCasesInt32ValueSlice {
		if in == nil {
			continue
		}
		out := Int32ValueSlice(in)
		assert.Len(t, out, len(in), "Unexpected len at idx %d", idx)
		for i := range out {
			if in[i] == nil {
				assert.Empty(t, out[i], "Unexpected value at idx %d", idx)
			} else {
				assert.Equal(t, *(in[i]), out[i], "Unexpected value at idx %d", idx)
			}
		}

		out2 := Int32Slice(out)
		assert.Len(t, out2, len(in), "Unexpected len at idx %d", idx)
		for i := range out2 {
			if in[i] == nil {
				assert.Empty(t, *(out2[i]), "Unexpected value at idx %d", idx)
			} else {
				assert.Equal(t, in[i], out2[i], "Unexpected value at idx %d", idx)
			}
		}
	}
}

var testCasesInt32Map = []map[string]int32{
	{"a": 3, "b": 2, "c": 1},
}

func TestInt32Map(t *testing.T) {
	for idx, in := range testCasesInt32Map {
		if in == nil {
			continue
		}
		out := Int32Map(in)
		assert.Len(t, out, len(in), "Unexpected len at idx %d", idx)
		for i := range out {
			assert.Equal(t, in[i], *(out[i]), "Unexpected value at idx %d", idx)
		}

		out2 := Int32ValueMap(out)
		assert.Len(t, out2, len(in), "Unexpected len at idx %d", idx)
		assert.Equal(t, in, out2, "Unexpected value at idx %d", idx)
	}
}

func TestInt64Value(t *testing.T) {
	i := int64(1024)
	assert.Equal(t, i, Int64Value(Int64(i)))

	assert.Equal(t, int64(0), Int64Value(nil))
}

var testCasesInt64Slice = [][]int64{
	{1, 2, 3, 4},
}

func TestInt64Slice(t *testing.T) {
	for idx, in := range testCasesInt64Slice {
		if in == nil {
			continue
		}
		out := Int64Slice(in)
		assert.Len(t, out, len(in), "Unexpected len at idx %d", idx)
		for i := range out {
			assert.Equal(t, in[i], *(out[i]), "Unexpected value at idx %d", idx)
		}

		out2 := Int64ValueSlice(out)
		assert.Len(t, out2, len(in), "Unexpected len at idx %d", idx)
		assert.Equal(t, in, out2, "Unexpected value at idx %d", idx)
	}
}

var testCasesInt64ValueSlice = [][]*int64{}

func TestInt64ValueSlice(t *testing.T) {
	for idx, in := range testCasesInt64ValueSlice {
		if in == nil {
			continue
		}
		out := Int64ValueSlice(in)
		assert.Len(t, out, len(in), "Unexpected len at idx %d", idx)
		for i := range out {
			if in[i] == nil {
				assert.Empty(t, out[i], "Unexpected value at idx %d", idx)
			} else {
				assert.Equal(t, *(in[i]), out[i], "Unexpected value at idx %d", idx)
			}
		}

		out2 := Int64Slice(out)
		assert.Len(t, out2, len(in), "Unexpected len at idx %d", idx)
		for i := range out2 {
			if in[i] == nil {
				assert.Empty(t, *(out2[i]), "Unexpected value at idx %d", idx)
			} else {
				assert.Equal(t, in[i], out2[i], "Unexpected value at idx %d", idx)
			}
		}
	}
}

var testCasesInt64Map = []map[string]int64{
	{"a": 3, "b": 2, "c": 1},
}

func TestInt64Map(t *testing.T) {
	for idx, in := range testCasesInt64Map {
		if in == nil {
			continue
		}
		out := Int64Map(in)
		assert.Len(t, out, len(in), "Unexpected len at idx %d", idx)
		for i := range out {
			assert.Equal(t, in[i], *(out[i]), "Unexpected value at idx %d", idx)
		}

		out2 := Int64ValueMap(out)
		assert.Len(t, out2, len(in), "Unexpected len at idx %d", idx)
		assert.Equal(t, in, out2, "Unexpected value at idx %d", idx)
	}
}

func TestUint8Value(t *testing.T) {
	i := uint8(128)
	assert.Equal(t, i, Uint8Value(Uint8(i)))

	assert.Equal(t, uint8(0), Uint8Value(nil))
}

func TestUint32Value(t *testing.T) {
	i := uint32(1024)
	assert.Equal(t, i, Uint32Value(Uint32(i)))

	assert.Equal(t, uint32(0), Uint32Value(nil))
}

func TestUint64Value(t *testing.T) {
	i := uint64(1024)
	assert.Equal(t, i, Uint64Value(Uint64(i)))

	assert.Equal(t, uint64(0), Uint64Value(nil))
}

var testCasesUint64Slice = [][]uint64{
	{1, 2, 3, 4},
}

func TestUint64Slice(t *testing.T) {
	for idx, in := range testCasesUint64Slice {
		if in == nil {
			continue
		}
		out := Uint64Slice(in)
		assert.Len(t, out, len(in), "Unexpected len at idx %d", idx)
		for i := range out {
			assert.Equal(t, in[i], *(out[i]), "Unexpected value at idx %d", idx)
		}

		out2 := Uint64ValueSlice(out)
		assert.Len(t, out2, len(in), "Unexpected len at idx %d", idx)
		assert.Equal(t, in, out2, "Unexpected value at idx %d", idx)
	}
}

func TestFloat32Value(t *testing.T) {
	i := float32(1024)
	assert.Equal(t, i, Float32Value(Float32(i)))

	assert.Equal(t, float32(0), Float32Value(nil))
}

var testCasesFloat32Slice = [][]float32{
	{1, 2, 3, 4},
}

func TestFloat32Slice(t *testing.T) {
	for idx, in := range testCasesFloat32Slice {
		if in == nil {
			continue
		}
		out := Float32Slice(in)
		assert.Len(t, out, len(in), "Unexpected len at idx %d", idx)
		for i := range out {
			assert.Equal(t, in[i], *(out[i]), "Unexpected value at idx %d", idx)
		}

		out2 := Float32ValueSlice(out)
		assert.Len(t, out2, len(in), "Unexpected len at idx %d", idx)
		assert.Equal(t, in, out2, "Unexpected value at idx %d", idx)
	}
}

var testCasesFloat32ValueSlice = [][]*float32{}

func TestFloat32ValueSlice(t *testing.T) {
	for idx, in := range testCasesFloat32ValueSlice {
		if in == nil {
			continue
		}
		out := Float32ValueSlice(in)
		assert.Len(t, out, len(in), "Unexpected len at idx %d", idx)
		for i := range out {
			if in[i] == nil {
				assert.Empty(t, out[i], "Unexpected value at idx %d", idx)
			} else {
				assert.Equal(t, *(in[i]), out[i], "Unexpected value at idx %d", idx)
			}
		}

		out2 := Float32Slice(out)
		assert.Len(t, out2, len(in), "Unexpected len at idx %d", idx)
		for i := range out2 {
			if in[i] == nil {
				assert.Empty(t, *(out2[i]), "Unexpected value at idx %d", idx)
			} else {
				assert.Equal(t, in[i], out2[i], "Unexpected value at idx %d", idx)
			}
		}
	}
}

var testCasesFloat32Map = []map[string]float32{
	{"a": 3, "b": 2, "c": 1},
}

func TestFloat32Map(t *testing.T) {
	for idx, in := range testCasesFloat32Map {
		if in == nil {
			continue
		}
		out := Float32Map(in)
		assert.Len(t, out, len(in), "Unexpected len at idx %d", idx)
		for i := range out {
			assert.Equal(t, in[i], *(out[i]), "Unexpected value at idx %d", idx)
		}

		out2 := Float32ValueMap(out)
		assert.Len(t, out2, len(in), "Unexpected len at idx %d", idx)
		assert.Equal(t, in, out2, "Unexpected value at idx %d", idx)
	}
}

func TestFloat64Value(t *testing.T) {
	i := float64(1024)
	assert.Equal(t, i, Float64Value(Float64(i)))

	assert.Equal(t, float64(0), Float64Value(nil))
}

var testCasesFloat64Slice = [][]float64{
	{1, 2, 3, 4},
}

func TestFloat64Slice(t *testing.T) {
	for idx, in := range testCasesFloat64Slice {
		if in == nil {
			continue
		}
		out := Float64Slice(in)
		assert.Len(t, out, len(in), "Unexpected len at idx %d", idx)
		for i := range out {
			assert.Equal(t, in[i], *(out[i]), "Unexpected value at idx %d", idx)
		}

		out2 := Float64ValueSlice(out)
		assert.Len(t, out2, len(in), "Unexpected len at idx %d", idx)
		assert.Equal(t, in, out2, "Unexpected value at idx %d", idx)
	}
}

var testCasesFloat64ValueSlice = [][]*float64{}

func TestFloat64ValueSlice(t *testing.T) {
	for idx, in := range testCasesFloat64ValueSlice {
		if in == nil {
			continue
		}
		out := Float64ValueSlice(in)
		assert.Len(t, out, len(in), "Unexpected len at idx %d", idx)
		for i := range out {
			if in[i] == nil {
				assert.Empty(t, out[i], "Unexpected value at idx %d", idx)
			} else {
				assert.Equal(t, *(in[i]), out[i], "Unexpected value at idx %d", idx)
			}
		}

		out2 := Float64Slice(out)
		assert.Len(t, out2, len(in), "Unexpected len at idx %d", idx)
		for i := range out2 {
			if in[i] == nil {
				assert.Empty(t, *(out2[i]), "Unexpected value at idx %d", idx)
			} else {
				assert.Equal(t, in[i], out2[i], "Unexpected value at idx %d", idx)
			}
		}
	}
}

var testCasesFloat64Map = []map[string]float64{
	{"a": 3, "b": 2, "c": 1},
}

func TestFloat64Map(t *testing.T) {
	for idx, in := range testCasesFloat64Map {
		if in == nil {
			continue
		}
		out := Float64Map(in)
		assert.Len(t, out, len(in), "Unexpected len at idx %d", idx)
		for i := range out {
			assert.Equal(t, in[i], *(out[i]), "Unexpected value at idx %d", idx)
		}

		out2 := Float64ValueMap(out)
		assert.Len(t, out2, len(in), "Unexpected len at idx %d", idx)
		assert.Equal(t, in, out2, "Unexpected value at idx %d", idx)
	}
}

func TestTimeValue(t *testing.T) {
	tm := time.Time{}
	assert.Equal(t, tm, TimeValue(Time(tm)))

	assert.Equal(t, time.Time{}, TimeValue(nil))
}

var testCasesTimeSlice = [][]time.Time{
	{time.Now(), time.Now().AddDate(100, 0, 0)},
}

func TestTimeSlice(t *testing.T) {
	for idx, in := range testCasesTimeSlice {
		if in == nil {
			continue
		}
		out := TimeSlice(in)
		assert.Len(t, out, len(in), "Unexpected len at idx %d", idx)
		for i := range out {
			assert.Equal(t, in[i], *(out[i]), "Unexpected value at idx %d", idx)
		}

		out2 := TimeValueSlice(out)
		assert.Len(t, out2, len(in), "Unexpected len at idx %d", idx)
		assert.Equal(t, in, out2, "Unexpected value at idx %d", idx)
	}
}

var testCasesTimeValueSlice = [][]*time.Time{}

func TestTimeValueSlice(t *testing.T) {
	for idx, in := range testCasesTimeValueSlice {
		if in == nil {
			continue
		}
		out := TimeValueSlice(in)
		assert.Len(t, out, len(in), "Unexpected len at idx %d", idx)
		for i := range out {
			if in[i] == nil {
				assert.Empty(t, out[i], "Unexpected value at idx %d", idx)
			} else {
				assert.Equal(t, *(in[i]), out[i], "Unexpected value at idx %d", idx)
			}
		}

		out2 := TimeSlice(out)
		assert.Len(t, out2, len(in), "Unexpected len at idx %d", idx)
		for i := range out2 {
			if in[i] == nil {
				assert.Empty(t, *(out2[i]), "Unexpected value at idx %d", idx)
			} else {
				assert.Equal(t, in[i], out2[i], "Unexpected value at idx %d", idx)
			}
		}
	}
}

var testCasesTimeMap = []map[string]time.Time{
	{"a": time.Now().AddDate(-100, 0, 0), "b": time.Now()},
}

func TestTimeMap(t *testing.T) {
	for idx, in := range testCasesTimeMap {
		if in == nil {
			continue
		}
		out := TimeMap(in)
		assert.Len(t, out, len(in), "Unexpected len at idx %d", idx)
		for i := range out {
			assert.Equal(t, in[i], *(out[i]), "Unexpected value at idx %d", idx)
		}

		out2 := TimeValueMap(out)
		assert.Len(t, out2, len(in), "Unexpected len at idx %d", idx)
		assert.Equal(t, in, out2, "Unexpected value at idx %d", idx)
	}
}
