package onedrive

import (
	"encoding/json"
	"testing"

	"github.com/rclone/rclone/backend/onedrive/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrderPermissions(t *testing.T) {
	tests := []struct {
		name     string
		input    []*api.PermissionsType
		expected []string
	}{
		{
			name:     "empty",
			input:    []*api.PermissionsType{},
			expected: []string(nil),
		},
		{
			name: "users first, then group, then none",
			input: []*api.PermissionsType{
				{ID: "1", GrantedTo: &api.IdentitySet{Group: api.Identity{DisplayName: "Group1"}}},
				{ID: "2", GrantedToIdentities: []*api.IdentitySet{{User: api.Identity{DisplayName: "Alice"}}}},
				{ID: "3", GrantedTo: &api.IdentitySet{User: api.Identity{DisplayName: "Alice"}}},
				{ID: "4"},
			},
			expected: []string{"2", "3", "1", "4"},
		},
		{
			name: "same type unsorted",
			input: []*api.PermissionsType{
				{ID: "b", GrantedTo: &api.IdentitySet{Group: api.Identity{DisplayName: "Group B"}}},
				{ID: "a", GrantedTo: &api.IdentitySet{Group: api.Identity{DisplayName: "Group A"}}},
				{ID: "c", GrantedToIdentities: []*api.IdentitySet{{Group: api.Identity{DisplayName: "Group A"}}, {User: api.Identity{DisplayName: "Alice"}}}},
			},
			expected: []string{"c", "b", "a"},
		},
		{
			name: "all user identities",
			input: []*api.PermissionsType{
				{ID: "c", GrantedTo: &api.IdentitySet{User: api.Identity{DisplayName: "Bob"}}},
				{ID: "a", GrantedTo: &api.IdentitySet{User: api.Identity{Email: "alice@example.com"}}},
				{ID: "b", GrantedToIdentities: []*api.IdentitySet{{User: api.Identity{LoginName: "user3"}}}},
			},
			expected: []string{"c", "a", "b"},
		},
		{
			name: "no user or group info",
			input: []*api.PermissionsType{
				{ID: "z"},
				{ID: "x"},
				{ID: "y"},
			},
			expected: []string{"z", "x", "y"},
		},
	}

	for _, driveType := range []string{driveTypePersonal, driveTypeBusiness} {
		t.Run(driveType, func(t *testing.T) {
			for _, tt := range tests {
				m := &Metadata{fs: &Fs{driveType: driveType}}
				t.Run(tt.name, func(t *testing.T) {
					if driveType == driveTypeBusiness {
						for i := range tt.input {
							tt.input[i].GrantedToV2 = tt.input[i].GrantedTo
							tt.input[i].GrantedTo = nil
							tt.input[i].GrantedToIdentitiesV2 = tt.input[i].GrantedToIdentities
							tt.input[i].GrantedToIdentities = nil
						}
					}
					m.orderPermissions(tt.input)
					var gotIDs []string
					for _, p := range tt.input {
						gotIDs = append(gotIDs, p.ID)
					}
					assert.Equal(t, tt.expected, gotIDs)
				})
			}
		})
	}
}

func TestOrderPermissionsJSON(t *testing.T) {
	testJSON := `[
  {
    "id": "1",
    "grantedToV2": {
      "group": {
        "id": "group@example.com"
      }
    },
    "roles": [
      "write"
    ]
  },
  {
    "id": "2",
    "grantedToV2": {
      "user": {
        "id": "user@example.com"
      }
    },
    "roles": [
      "write"
    ]
  }
]`

	var testPerms []*api.PermissionsType
	err := json.Unmarshal([]byte(testJSON), &testPerms)
	require.NoError(t, err)

	m := &Metadata{fs: &Fs{driveType: driveTypeBusiness}}
	m.orderPermissions(testPerms)
	var gotIDs []string
	for _, p := range testPerms {
		gotIDs = append(gotIDs, p.ID)
	}
	assert.Equal(t, []string{"2", "1"}, gotIDs)

}
