package backend

import (
	"reflect"
	"testing"
)

func Test_mergeUpdates(t *testing.T) {
	tests := []struct {
		name string
		have []update
		want []update
	}{
		{
			name: "single",
			have: []update{&labelCreated{labelID: "1"}},
			want: []update{&labelCreated{labelID: "1"}},
		},
		{
			name: "multiple",
			have: []update{
				&labelCreated{labelID: "1"},
				&labelCreated{labelID: "2"},
			},
			want: []update{
				&labelCreated{labelID: "1"},
				&labelCreated{labelID: "2"},
			},
		},
		{
			name: "replace with updated",
			have: []update{
				&labelCreated{labelID: "1"},
				&labelUpdated{labelID: "1"},
				&labelUpdated{labelID: "1"},
			},
			want: []update{
				&labelCreated{labelID: "1"},
				&labelUpdated{labelID: "1"},
			},
		},
		{
			name: "replace with delete",
			have: []update{
				&labelCreated{labelID: "1"},
				&labelUpdated{labelID: "1"},
				&labelUpdated{labelID: "1"},
				&labelDeleted{labelID: "1"},
			},
			want: []update{
				&labelDeleted{labelID: "1"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := merge(tt.have); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("mergeUpdates() = %v, want %v", got, tt.want)
			}
		})
	}
}
