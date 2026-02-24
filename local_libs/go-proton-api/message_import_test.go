package proton

import (
	"reflect"
	"testing"
)

func Test_chunkSized(t *testing.T) {
	type args struct {
		vals    []int
		maxLen  int
		maxSize int
		getSize func(int) int
	}

	tests := []struct {
		name string
		args args
		want [][]int
	}{
		{
			name: "limit by length",
			args: args{
				vals:    []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
				maxLen:  3, // Split into chunks of at most 3
				maxSize: 100,
				getSize: func(i int) int { return i },
			},
			want: [][]int{
				{1, 2, 3},
				{4, 5, 6},
				{7, 8, 9},
				{10},
			},
		},
		{
			name: "limit by size",
			args: args{
				vals:    []int{1, 1, 1, 1, 1, 2, 2, 2, 2, 2},
				maxLen:  100,
				maxSize: 5, // Split into chunks of at most 5
				getSize: func(i int) int { return i },
			},
			want: [][]int{
				{1, 1, 1, 1, 1},
				{2, 2},
				{2, 2},
				{2},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := chunkSized(tt.args.vals, tt.args.maxLen, tt.args.maxSize, tt.args.getSize); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("chunkSized() = %v, want %v", got, tt.want)
			}
		})
	}
}
