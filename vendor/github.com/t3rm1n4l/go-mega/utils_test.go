package mega

import (
	"reflect"
	"testing"
)

func TestGetChunkSizes(t *testing.T) {
	const k = 1024
	for _, test := range []struct {
		size int64
		want []chunkSize
	}{
		{
			size: 0,
			want: []chunkSize(nil),
		},
		{
			size: 1,
			want: []chunkSize{
				{0, 1},
			},
		},
		{
			size: 128*k - 1,
			want: []chunkSize{
				{0, 128*k - 1},
			},
		},
		{
			size: 128 * k,
			want: []chunkSize{
				{0, 128 * k},
			},
		},
		{
			size: 128*k + 1,
			want: []chunkSize{
				{0, 128 * k},
				{128 * k, 1},
			},
		},
		{
			size: 384*k - 1,
			want: []chunkSize{
				{0, 128 * k},
				{128 * k, 256*k - 1},
			},
		},
		{
			size: 384 * k,
			want: []chunkSize{
				{0, 128 * k},
				{128 * k, 256 * k},
			},
		},
		{
			size: 384*k + 1,
			want: []chunkSize{
				{0, 128 * k},
				{128 * k, 256 * k},
				{384 * k, 1},
			},
		},
		{
			size: 5 * k * k,
			want: []chunkSize{
				{0, 128 * k},
				{128 * k, 256 * k},
				{384 * k, 384 * k},
				{768 * k, 512 * k},
				{1280 * k, 640 * k},
				{1920 * k, 768 * k},
				{2688 * k, 896 * k},
				{3584 * k, 1024 * k},
				{4608 * k, 512 * k},
			},
		},
		{
			size: 10 * k * k,
			want: []chunkSize{
				{0, 128 * k},
				{128 * k, 256 * k},
				{384 * k, 384 * k},
				{768 * k, 512 * k},
				{1280 * k, 640 * k},
				{1920 * k, 768 * k},
				{2688 * k, 896 * k},
				{3584 * k, 1024 * k},
				{4608 * k, 1024 * k},
				{5632 * k, 1024 * k},
				{6656 * k, 1024 * k},
				{7680 * k, 1024 * k},
				{8704 * k, 1024 * k},
				{9728 * k, 512 * k},
			},
		},
	} {
		got := getChunkSizes(test.size)
		if !reflect.DeepEqual(test.want, got) {
			t.Errorf("incorrect chunks for size %d: want %#v, got %#v", test.size, test.want, got)

		}
	}
}
