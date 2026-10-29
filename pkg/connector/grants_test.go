package connector

import (
	"context"
	"sort"
	"testing"

	"github.com/conductorone/baton-mysql/pkg/client"
	"github.com/stretchr/testify/require"
)

func TestRoutineGrantKeys(t *testing.T) {
	tests := []struct {
		name    string
		grants  []*client.RoutineGrant
		skipDbs map[string]struct{}
		want    []string
	}{
		{
			name: "single execute grant",
			grants: []*client.RoutineGrant{
				{Id: "routine:db1.p1", Database: "db1", Privs: "Execute"},
			},
			want: []string{"execute:routine:db1.p1"},
		},
		{
			name: "skipped db omitted",
			grants: []*client.RoutineGrant{
				{Id: "routine:db1.p1", Database: "db1", Privs: "Execute"},
				{Id: "routine:skipme.p1", Database: "skipme", Privs: "Execute"},
			},
			skipDbs: map[string]struct{}{"skipme": {}},
			want:    []string{"execute:routine:db1.p1"},
		},
		{
			name:   "empty input",
			grants: nil,
			want:   []string{},
		},
		{
			name: "multiple privs produce one key per decoded priv",
			grants: []*client.RoutineGrant{
				{Id: "routine:db1.p1", Database: "db1", Privs: "Execute,Alter Routine,Grant"},
			},
			want: []string{
				"execute:routine:db1.p1",
				"alter_routine:routine:db1.p1",
				"grant:routine:db1.p1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := routineGrantKeys(context.Background(), tt.grants, tt.skipDbs)
			// GetPrivs returns a map, so compare as sorted sets, not positional order.
			sort.Strings(got)
			want := append([]string(nil), tt.want...)
			sort.Strings(want)
			require.Equal(t, want, got)
		})
	}
}
