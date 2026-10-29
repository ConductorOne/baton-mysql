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
			name:      "plain membership",
			id:        "role:ops@team@%",
			withAdmin: false,
			want:      []string{"role_assignment:role:ops@team@%"},
		},
		{
			name:      "with admin option: with_grant key is ADDITIONAL, never a replacement",
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
	grant := buildGrant(
		"role_assignment:role:ops@team@%",
		&v2.ResourceId{ResourceType: "user", Resource: "user:joshua@h2"},
	)

	require.Equal(t, "entitlement:role_assignment:role:ops@team@%", grant.Entitlement.Id)
	require.Equal(t, &v2.ResourceId{ResourceType: "role", Resource: "role:ops@team@%"}, grant.Entitlement.Resource.Id)
	require.Equal(t, &v2.ResourceId{ResourceType: "user", Resource: "user:joshua@h2"}, grant.Principal.Id)
	// Literal string equality — the provisioner-shaped ID roleSyncer.Revoke parses.
	require.Equal(t, "grant:entitlement:role_assignment:role:ops@team@%:user:joshua@h2", grant.Id)
}

func TestBuildGrantRoleHierarchy(t *testing.T) {
	grant := buildGrant(
		"role_assignment:role:app_reader@%",
		&v2.ResourceId{ResourceType: "role", Resource: "role:ops@team@%"},
	)

	require.Equal(t, "entitlement:role_assignment:role:app_reader@%", grant.Entitlement.Id)
	require.Equal(t, &v2.ResourceId{ResourceType: "role", Resource: "role:app_reader@%"}, grant.Entitlement.Resource.Id)
	require.Equal(t, "role", grant.Principal.Id.ResourceType)
	require.Equal(t, "grant:entitlement:role_assignment:role:app_reader@%:role:ops@team@%", grant.Id)
}

func TestBuildGrantCollapsedUsers(t *testing.T) {
	collapsedID := &v2.ResourceId{ResourceType: "user", Resource: "user:plain_user@%,localhost"}
	grant := buildGrant(
		"role_assignment:role:ops@team@%",
		collapsedID,
	)

	require.Equal(t, collapsedID, grant.Principal.Id)
	require.NotContains(t, fmt.Sprintf("%s|%s", grant.Id, grant.Principal.Id.Resource), "user:plain_user@localhost")
}