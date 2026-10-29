package client

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRoutineGrantGetPrivs(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name  string
		privs string
		want  map[string]struct{}
	}{
		{
			name:  "execute case-preserved",
			privs: "Execute",
			want:  map[string]struct{}{"Execute": {}},
		},
		{
			name:  "alter routine gets underscore, case preserved",
			privs: "Alter Routine",
			want:  map[string]struct{}{"Alter_Routine": {}},
		},
		{
			name:  "grant",
			privs: "Grant",
			want:  map[string]struct{}{"Grant": {}},
		},
		{
			name:  "multiple privs",
			privs: "Execute,Alter Routine",
			want:  map[string]struct{}{"Execute": {}, "Alter_Routine": {}},
		},
		{
			name:  "empty string -> empty map",
			privs: "",
			want:  map[string]struct{}{},
		},
		{
			name:  "unknown priv decoded verbatim",
			privs: "Execute,Bogus",
			want:  map[string]struct{}{"Execute": {}, "Bogus": {}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := &RoutineGrant{Privs: tt.privs}
			require.Equal(t, tt.want, g.GetPrivs(ctx))
		})
	}
}

func TestRoutineGrantIDShape(t *testing.T) {
	g := &RoutineGrant{Database: "db1", RoutineName: "p1"}
	g.Id = dbResourceID{
		ResourceTypeID: RoutineType,
		DatabaseName:   g.Database,
		ResourceName:   g.RoutineName,
	}.String()
	require.Equal(t, "routine:db1.p1", g.Id)
}