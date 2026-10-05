package connector

import (
	"testing"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/stretchr/testify/require"

	"github.com/conductorone/baton-mysql/pkg/client"
)

func TestPrincipalAccount(t *testing.T) {
	tests := []struct {
		name    string
		id      *v2.ResourceId
		want    string
		wantErr bool
	}{
		{
			name: "user",
			id:   &v2.ResourceId{ResourceType: resourceTypeUser.Id, Resource: "user:alice@localhost"},
			want: "alice@localhost",
		},
		{
			name: "user with email username",
			id:   &v2.ResourceId{ResourceType: resourceTypeUser.Id, Resource: "user:jane.doe@example.com@%"},
			want: "jane.doe@example.com@%",
		},
		{
			name: "role",
			id:   &v2.ResourceId{ResourceType: resourceTypeRole.Id, Resource: "role:app_reader@%"},
			want: "app_reader@%",
		},
		{
			name: "username containing colon",
			id:   &v2.ResourceId{ResourceType: resourceTypeUser.Id, Resource: "user:svc:etl@%"},
			want: "svc:etl@%",
		},
		{
			name:    "missing type prefix",
			id:      &v2.ResourceId{ResourceType: resourceTypeUser.Id, Resource: "alice@localhost"},
			wantErr: true,
		},
		{
			name:    "prefix of a different type",
			id:      &v2.ResourceId{ResourceType: resourceTypeUser.Id, Resource: "role:alice@localhost"},
			wantErr: true,
		},
		{
			name:    "empty account",
			id:      &v2.ResourceId{ResourceType: resourceTypeUser.Id, Resource: "user:"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := principalAccount(tt.id)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestPrincipalIDRoundTrip(t *testing.T) {
	users := []*client.User{
		{UserType: client.UserType, User: "jane.doe@example.com", Host: "%"},
		{UserType: client.UserType, User: "a@b@c", Host: "localhost"},
		{UserType: client.RoleType, User: "", Host: "localhost"},
		{UserType: client.UserType, User: "jane.doe@example.com", Host: "localhost,%"},
	}

	for _, u := range users {
		t.Run(u.GetID(), func(t *testing.T) {
			account, err := principalAccount(&v2.ResourceId{ResourceType: u.UserType, Resource: u.GetID()})
			require.NoError(t, err)

			user, host, err := client.SplitUserHost(account)
			require.NoError(t, err)
			require.Equal(t, u.User, user)
			require.Equal(t, u.Host, host)
		})
	}
}
