package client

import (
	"database/sql"
	"errors"
	"fmt"
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
			name: "same user, different host — not a self edge, must sync",
			edge: &RoleGrant{FromUser: "dup", FromHost: "%", ToUser: "dup", ToHost: "localhost"},
			want: false,
		},
		{
			name: "same user and same host — self edge, dropped",
			edge: &RoleGrant{FromUser: "dup", FromHost: "%", ToUser: "dup", ToHost: "%"},
			want: true,
		},
		{
			name: "differing user",
			edge: &RoleGrant{FromUser: "a", FromHost: "%", ToUser: "b", ToHost: "%"},
			want: false,
		},
		{
			name: "differing host",
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

func TestIsAccountNotFoundErr(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "bare sentinel", err: sql.ErrNoRows, want: true},
		{name: "wrapped sentinel", err: fmt.Errorf("baton-mysql: fetch failed: %w", sql.ErrNoRows), want: true},
		{name: "unrelated error", err: errors.New("permission denied"), want: false},
		{name: "nil", err: nil, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, isAccountNotFoundErr(tt.err))
		})
	}
}

func TestListRoleGrantsQueryPredicatesOnGrantee(t *testing.T) {
	// The corrected direction: the synced account is the GRANTEE (TO side).
	// Guards against a regression to the transposed FROM-side predicate.
	require.Contains(t, roleGrantsQuery, "WHERE TO_USER = ? AND TO_HOST = ?")
	require.NotContains(t, roleGrantsQuery, "WHERE FROM_USER = ?")
}
