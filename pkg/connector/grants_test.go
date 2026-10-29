package connector

import (
	"fmt"
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
			name:      "membership only",
			id:        "role:ops@team@%",
			withAdmin: false,
			want:      []string{"role_assignment:role:ops@team@%"},
		},
		{
			name:      "with grant is additional",
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
	require.Equal(t, "role", g.Entitlement.Resource.Id.ResourceType)
	require.Equal(t, "role:ops@team@%", g.Entitlement.Resource.Id.Resource)
	require.Equal(t, "user", g.Principal.Id.ResourceType)
	require.Equal(t, "user:joshua@h2", g.Principal.Id.Resource)
	require.Equal(t, "grant:entitlement:role_assignment:role:ops@team@%:user:joshua@h2", g.Id)
}

func TestBuildGrantRoleHierarchy(t *testing.T) {
	g := buildGrant("role_assignment:role:app_reader@%", &v2.ResourceId{ResourceType: "role", Resource: "role:ops@team@%"})

	require.Equal(t, "entitlement:role_assignment:role:app_reader@%", g.Entitlement.Id)
	require.Equal(t, "role", g.Principal.Id.ResourceType)
	require.Equal(t, "role:ops@team@%", g.Principal.Id.Resource)
	require.Equal(t, "grant:entitlement:role_assignment:role:app_reader@%:role:ops@team@%", g.Id)
}

func TestBuildGrantCollapsedUsers(t *testing.T) {
	g := buildGrant("role_assignment:role:app_reader@%", &v2.ResourceId{ResourceType: "user", Resource: "user:plain_user@%,localhost"})

	require.Equal(t, &v2.ResourceId{ResourceType: "user", Resource: "user:plain_user@%,localhost"}, g.Principal.Id)
	require.NotContains(t, fmt.Sprintf("%s:%s", g.Id, g.Principal.Id.Resource), "user:plain_user@localhost")
}