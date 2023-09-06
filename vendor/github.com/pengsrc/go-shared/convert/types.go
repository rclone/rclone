package convert

import (
	"time"
)

// String returns a pointer to the given string value.
func String(v string) *string {
	return &v
}

// StringValue returns the value of the given string pointer or
// "" if the pointer is nil.
func StringValue(v *string) string {
	if v != nil {
		return *v
	}
	return ""
}

// StringSlice converts a slice of string values into a slice of
// string pointers.
func StringSlice(src []string) []*string {
	dst := make([]*string, len(src))
	for i := 0; i < len(src); i++ {
		dst[i] = &(src[i])
	}
	return dst
}

// StringValueSlice converts a slice of string pointers into a slice of
// string values.
func StringValueSlice(src []*string) []string {
	dst := make([]string, len(src))
	for i := 0; i < len(src); i++ {
		if src[i] != nil {
			dst[i] = *(src[i])
		}
	}
	return dst
}

// StringMap converts a string map of string values into a string
// map of string pointers.
func StringMap(src map[string]string) map[string]*string {
	dst := make(map[string]*string)
	for k, val := range src {
		v := val
		dst[k] = &v
	}
	return dst
}

// StringValueMap converts a string map of string pointers into a string
// map of string values.
func StringValueMap(src map[string]*string) map[string]string {
	dst := make(map[string]string)
	for k, val := range src {
		if val != nil {
			dst[k] = *val
		}
	}
	return dst
}

// Bool returns a pointer to the given bool value.
func Bool(v bool) *bool {
	return &v
}

// BoolValue returns the value of the given bool pointer or
// false if the pointer is nil.
func BoolValue(v *bool) bool {
	if v != nil {
		return *v
	}
	return false
}

// BoolSlice converts a slice of bool values into a slice of
// bool pointers.
func BoolSlice(src []bool) []*bool {
	dst := make([]*bool, len(src))
	for i := 0; i < len(src); i++ {
		dst[i] = &(src[i])
	}
	return dst
}

// BoolValueSlice converts a slice of bool pointers into a slice of
// bool values.
func BoolValueSlice(src []*bool) []bool {
	dst := make([]bool, len(src))
	for i := 0; i < len(src); i++ {
		if src[i] != nil {
			dst[i] = *(src[i])
		}
	}
	return dst
}

// BoolMap converts a string map of bool values into a string
// map of bool pointers.
func BoolMap(src map[string]bool) map[string]*bool {
	dst := make(map[string]*bool)
	for k, val := range src {
		v := val
		dst[k] = &v
	}
	return dst
}

// BoolValueMap converts a string map of bool pointers into a string
// map of bool values.
func BoolValueMap(src map[string]*bool) map[string]bool {
	dst := make(map[string]bool)
	for k, val := range src {
		if val != nil {
			dst[k] = *val
		}
	}
	return dst
}

// Int returns a pointer to the given int value.
func Int(v int) *int {
	return &v
}

// IntValue returns the value of the given int pointer or
// 0 if the pointer is nil.
func IntValue(v *int) int {
	if v != nil {
		return *v
	}
	return 0
}

// IntSlice converts a slice of int values into a slice of
// int pointers.
func IntSlice(src []int) []*int {
	dst := make([]*int, len(src))
	for i := 0; i < len(src); i++ {
		dst[i] = &(src[i])
	}
	return dst
}

// IntValueSlice converts a slice of int pointers into a slice of
// int values.
func IntValueSlice(src []*int) []int {
	dst := make([]int, len(src))
	for i := 0; i < len(src); i++ {
		if src[i] != nil {
			dst[i] = *(src[i])
		}
	}
	return dst
}

// IntMap converts a string map of int values into a string
// map of int pointers.
func IntMap(src map[string]int) map[string]*int {
	dst := make(map[string]*int)
	for k, val := range src {
		v := val
		dst[k] = &v
	}
	return dst
}

// IntValueMap converts a string map of int pointers into a string
// map of int values.
func IntValueMap(src map[string]*int) map[string]int {
	dst := make(map[string]int)
	for k, val := range src {
		if val != nil {
			dst[k] = *val
		}
	}
	return dst
}

// Int32 returns a pointer to the given int32 value.
func Int32(v int32) *int32 {
	return &v
}

// Int32Value returns the value of the given int32 pointer or
// 0 if the pointer is nil.
func Int32Value(v *int32) int32 {
	if v != nil {
		return *v
	}
	return 0
}

// Int32Slice converts a slice of int32 values into a slice of
// int32 pointers.
func Int32Slice(src []int32) []*int32 {
	dst := make([]*int32, len(src))
	for i := 0; i < len(src); i++ {
		dst[i] = &(src[i])
	}
	return dst
}

// Int32ValueSlice converts a slice of int32 pointers into a slice of
// int32 values.
func Int32ValueSlice(src []*int32) []int32 {
	dst := make([]int32, len(src))
	for i := 0; i < len(src); i++ {
		if src[i] != nil {
			dst[i] = *(src[i])
		}
	}
	return dst
}

// Int32Map converts a string map of int32 values into a string
// map of int32 pointers.
func Int32Map(src map[string]int32) map[string]*int32 {
	dst := make(map[string]*int32)
	for k, val := range src {
		v := val
		dst[k] = &v
	}
	return dst
}

// Int32ValueMap converts a string map of int32 pointers into a string
// map of int32 values.
func Int32ValueMap(src map[string]*int32) map[string]int32 {
	dst := make(map[string]int32)
	for k, val := range src {
		if val != nil {
			dst[k] = *val
		}
	}
	return dst
}

// Int64 returns a pointer to the given int64 value.
func Int64(v int64) *int64 {
	return &v
}

// Int64Value returns the value of the given int64 pointer or
// 0 if the pointer is nil.
func Int64Value(v *int64) int64 {
	if v != nil {
		return *v
	}
	return 0
}

// Int64Slice converts a slice of int64 values into a slice of
// int64 pointers.
func Int64Slice(src []int64) []*int64 {
	dst := make([]*int64, len(src))
	for i := 0; i < len(src); i++ {
		dst[i] = &(src[i])
	}
	return dst
}

// Int64ValueSlice converts a slice of int64 pointers into a slice of
// int64 values.
func Int64ValueSlice(src []*int64) []int64 {
	dst := make([]int64, len(src))
	for i := 0; i < len(src); i++ {
		if src[i] != nil {
			dst[i] = *(src[i])
		}
	}
	return dst
}

// Int64Map converts a string map of int64 values into a string
// map of int64 pointers.
func Int64Map(src map[string]int64) map[string]*int64 {
	dst := make(map[string]*int64)
	for k, val := range src {
		v := val
		dst[k] = &v
	}
	return dst
}

// Int64ValueMap converts a string map of int64 pointers into a string
// map of int64 values.
func Int64ValueMap(src map[string]*int64) map[string]int64 {
	dst := make(map[string]int64)
	for k, val := range src {
		if val != nil {
			dst[k] = *val
		}
	}
	return dst
}

// Int64Uint returns a uint pointer to the given int64 value.
func Int64Uint(src int64) *uint {
	dst := uint(src)
	return &dst
}

// Uint8 return a uint8 pointer to the given uint8 value.
func Uint8(src uint8) *uint8 {
	dst := uint8(src)
	return &dst
}

// Uint8Value returns the value of the given uint8 pointer or
// 0 if the pointer is nil.
func Uint8Value(v *uint8) uint8 {
	if v != nil {
		return *v
	}
	return 0
}

// Uint32 return a uint32 pointer to the given uint32 value.
func Uint32(src uint32) *uint32 {
	dst := uint32(src)
	return &dst
}

// Uint32Value returns the value of the given uint32 pointer or
// 0 if the pointer is nil.
func Uint32Value(v *uint32) uint32 {
	if v != nil {
		return *v
	}
	return 0
}

// Uint64 return a uint64 pointer to the given uint64 value.
func Uint64(src uint64) *uint64 {
	dst := uint64(src)
	return &dst
}

// Uint64Value returns the value of the given uint64 pointer or
// 0 if the pointer is nil.
func Uint64Value(v *uint64) uint64 {
	if v != nil {
		return *v
	}
	return 0
}

// Uint64Slice converts a slice of uint64 values into a slice of
// uint64 pointers.
func Uint64Slice(src []uint64) []*uint64 {
	dst := make([]*uint64, len(src))
	for i := 0; i < len(src); i++ {
		dst[i] = &(src[i])
	}
	return dst
}

// Uint64ValueSlice converts a slice of uint64 pointers into a slice of
// uint64 values.
func Uint64ValueSlice(src []*uint64) []uint64 {
	dst := make([]uint64, len(src))
	for i := 0; i < len(src); i++ {
		if src[i] != nil {
			dst[i] = *(src[i])
		}
	}
	return dst
}

// Float32 returns a pointer to the given float32 value.
func Float32(v float32) *float32 {
	return &v
}

// Float32Value returns the value of the given float32 pointer or
// 0 if the pointer is nil.
func Float32Value(v *float32) float32 {
	if v != nil {
		return *v
	}
	return 0
}

// Float32Slice converts a slice of float32 values into a slice of
// float32 pointers.
func Float32Slice(src []float32) []*float32 {
	dst := make([]*float32, len(src))
	for i := 0; i < len(src); i++ {
		dst[i] = &(src[i])
	}
	return dst
}

// Float32ValueSlice converts a slice of float32 pointers into a slice of
// float32 values.
func Float32ValueSlice(src []*float32) []float32 {
	dst := make([]float32, len(src))
	for i := 0; i < len(src); i++ {
		if src[i] != nil {
			dst[i] = *(src[i])
		}
	}
	return dst
}

// Float32Map converts a string map of float32 values into a string
// map of float32 pointers.
func Float32Map(src map[string]float32) map[string]*float32 {
	dst := make(map[string]*float32)
	for k, val := range src {
		v := val
		dst[k] = &v
	}
	return dst
}

// Float32ValueMap converts a string map of float32 pointers into a string
// map of float32 values.
func Float32ValueMap(src map[string]*float32) map[string]float32 {
	dst := make(map[string]float32)
	for k, val := range src {
		if val != nil {
			dst[k] = *val
		}
	}
	return dst
}

// Float64 returns a pointer to the given float64 value.
func Float64(v float64) *float64 {
	return &v
}

// Float64Value returns the value of the given float64 pointer or
// 0 if the pointer is nil.
func Float64Value(v *float64) float64 {
	if v != nil {
		return *v
	}
	return 0
}

// Float64Slice converts a slice of float64 values into a slice of
// float64 pointers.
func Float64Slice(src []float64) []*float64 {
	dst := make([]*float64, len(src))
	for i := 0; i < len(src); i++ {
		dst[i] = &(src[i])
	}
	return dst
}

// Float64ValueSlice converts a slice of float64 pointers into a slice of
// float64 values.
func Float64ValueSlice(src []*float64) []float64 {
	dst := make([]float64, len(src))
	for i := 0; i < len(src); i++ {
		if src[i] != nil {
			dst[i] = *(src[i])
		}
	}
	return dst
}

// Float64Map converts a string map of float64 values into a string
// map of float64 pointers.
func Float64Map(src map[string]float64) map[string]*float64 {
	dst := make(map[string]*float64)
	for k, val := range src {
		v := val
		dst[k] = &v
	}
	return dst
}

// Float64ValueMap converts a string map of float64 pointers into a string
// map of float64 values.
func Float64ValueMap(src map[string]*float64) map[string]float64 {
	dst := make(map[string]float64)
	for k, val := range src {
		if val != nil {
			dst[k] = *val
		}
	}
	return dst
}

// Time returns a pointer to the given time.Time value.
func Time(v time.Time) *time.Time {
	return &v
}

// TimeValue returns the value of the given time.Time pointer or
// time.Time{} if the pointer is nil.
func TimeValue(v *time.Time) time.Time {
	if v != nil {
		return *v
	}
	return time.Time{}
}

// TimeSlice converts a slice of time.Time values into a slice of
// time.Time pointers.
func TimeSlice(src []time.Time) []*time.Time {
	dst := make([]*time.Time, len(src))
	for i := 0; i < len(src); i++ {
		dst[i] = &(src[i])
	}
	return dst
}

// TimeValueSlice converts a slice of time.Time pointers into a slice of
// time.Time values.
func TimeValueSlice(src []*time.Time) []time.Time {
	dst := make([]time.Time, len(src))
	for i := 0; i < len(src); i++ {
		if src[i] != nil {
			dst[i] = *(src[i])
		}
	}
	return dst
}

// TimeMap converts a string map of time.Time values into a string
// map of time.Time pointers.
func TimeMap(src map[string]time.Time) map[string]*time.Time {
	dst := make(map[string]*time.Time)
	for k, val := range src {
		v := val
		dst[k] = &v
	}
	return dst
}

// TimeValueMap converts a string map of time.Time pointers into a string
// map of time.Time values.
func TimeValueMap(src map[string]*time.Time) map[string]time.Time {
	dst := make(map[string]time.Time)
	for k, val := range src {
		if val != nil {
			dst[k] = *val
		}
	}
	return dst
}
