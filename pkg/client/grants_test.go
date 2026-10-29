package client

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsSelfRoleEdge(t *testing.T) {
	tests := []struct {
		name string
		edge *RoleGrant
		want bool
	}{
		{
			name: "same user different host is NOT a self edge (must sync)",
			edge: &RoleGrant{FromUser: "dup", FromHost: "%", ToUser: "dup", ToHost: "localhost"},
			want: false,
		},
		{
			name: "same user same host is a self edge (dropped)",
			edge: &RoleGrant{FromUser: "dup", FromHost: "%", ToUser: "dup", ToHost: "%"},
			want: true,
		},
		{
			name: "differing user same host",
			edge: &RoleGrant{FromUser: "a", FromHost: "%", ToUser: "b", ToHost: "%"},
			want: false,
		},
		{
			name: "same user differing host (h2)",
			edge: &RoleGrant{FromUser: "dup", FromHost: "%", ToUser: "dup", ToHost: "h2"},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, isSelfRoleEdge(tt.edge))
		})
	}
}