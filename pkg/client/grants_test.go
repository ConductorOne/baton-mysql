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
			// Same user, different host: NOT a self edge — MUST sync (AC4).
			name: "same user different host",
			edge: &RoleGrant{FromUser: "dup", FromHost: "%", ToUser: "dup", ToHost: "localhost"},
			want: false,
		},
		{
			// Same user AND same host: a true self edge — dropped.
			name: "same user same host",
			edge: &RoleGrant{FromUser: "dup", FromHost: "%", ToUser: "dup", ToHost: "%"},
			want: true,
		},
		{
			name: "differing user same host",
			edge: &RoleGrant{FromUser: "a", FromHost: "%", ToUser: "b", ToHost: "%"},
			want: false,
		},
		{
			name: "differing host same user",
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
