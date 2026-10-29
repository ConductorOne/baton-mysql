package connector

import (
	"testing"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/stretchr/testify/require"
)

func TestRoleGrantKeys(t *testing.T) {
	tests := []struct {
		name      string
		id        string
		withAdmin bool
		want      []string
	}{
		{
			name:      "membership key only without admin option",
			id:        "role:ops@team@%",
			withAdmin: false,
			want:      []string{"role_assignment:role:ops@team@%"},
		},
		{
			name:      "with_grant key is additional, never a replacement",
			id:        "role:ops@team@%",
			withAdmin: true,
			want:      []string{"role_assignment:role:ops@team@%", "role_assignment_with_grant:role:ops@team@%"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, roleGrantKeys(tt.id, tt.withAdmin))
		})
	}
}

func TestBuildGrantMembership(t *testing.T) {
	g := buildGrant("role_assignment:role:ops@team@%", &v2.ResourceId{ResourceType: "user", Resource: "user:joshua@h2"})

	require.Equal(t, "entitlement:role_assignment:role:ops@team@%", g.Entitlement.Id)
	require.Equal(t, &v2.ResourceId{ResourceType: "role", Resource: "role:ops@team@%"}, g.Entitlement.Resource.Id)
	require.Equal(t, &v2.ResourceId{ResourceType: "user", Resource: "user:joshua@h2"}, g.Principal.Id)
	// Literal provisioner-shaped ID that roleSyncer.Revoke parses.
	require.Equal(t, "grant:entitlement:role_assignment:role:ops@team@%:user:joshua@h2", g.Id)
}

func TestBuildGrantRoleHierarchy(t *testing.T) {
	g := buildGrant("role_assignment:role:app_reader@%", &v2.ResourceId{ResourceType: "role", Resource: "role:ops@team@%"})

	require.Equal(t, "entitlement:role_assignment:role:app_reader@%", g.Entitlement.Id)
	require.Equal(t, &v2.ResourceId{ResourceType: "role", Resource: "role:app_reader@%"}, g.Entitlement.Resource.Id)
	require.Equal(t, "role", g.Principal.Id.ResourceType)
	require.Equal(t, "grant:entitlement:role_assignment:role:app_reader@%:role:ops@team@%", g.Id)
}

func TestBuildGrantCollapsedUsers(t *testing.T) {
	g := buildGrant("role_assignment:role:app_reader@%", &v2.ResourceId{ResourceType: "user", Resource: "user:plain_user@%,localhost"})

	require.Equal(t, &v2.ResourceId{ResourceType: "user", Resource: "user:plain_user@%,localhost"}, g.Principal.Id)
	require.NotContains(t, g.Id, "user:plain_user@localhost")
	require.NotContains(t, g.Principal.Id.Resource, "user:plain_user@localhost")
}