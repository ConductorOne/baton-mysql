package connector

import (
	"fmt"
	"testing"

	"github.com/conductorone/baton-mysql/pkg/client"
	"github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/require"
)

func TestIsNoSuchGrantErr(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "mysql error 1147",
			err:  &mysql.MySQLError{Number: 1147},
			want: true,
		},
		{
			name: "mysql error 1403 (routine-level no such grant)",
			err:  &mysql.MySQLError{Number: 1403},
			want: true,
		},
		{
			name: "mysql error 1064",
			err:  &mysql.MySQLError{Number: 1064},
			want: false,
		},
		{
			name: "wrapped mysql error 1147",
			err:  fmt.Errorf("wrapped: %w", &mysql.MySQLError{Number: 1147}),
			want: true,
		},
		{
			name: "wrapped mysql error 1403",
			err:  fmt.Errorf("wrapped: %w", &mysql.MySQLError{Number: 1403}),
			want: true,
		},
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, isNoSuchGrantErr(tt.err))
		})
	}
}

func TestCanonicalRoutineMap(t *testing.T) {
	tests := []struct {
		name string
		rows []client.RoutineSchemaRow
		want map[string]string
	}{
		{
			name: "canonical case wins over procs_priv case",
			rows: []client.RoutineSchemaRow{
				{Schema: "db1", Name: "p1"},
				{Schema: "db1", Name: "P2"},
			},
			want: map[string]string{
				"db1\x00p1": "p1",
				"db1\x00p2": "P2",
			},
		},
		{
			name: "lookup is case-insensitive on schema and name",
			rows: []client.RoutineSchemaRow{{Schema: "Db1", Name: "P1"}},
			want: map[string]string{"db1\x00p1": "P1"},
		},
		{
			name: "same routine name in two schemas kept apart",
			rows: []client.RoutineSchemaRow{
				{Schema: "db1", Name: "p1"},
				{Schema: "db2", Name: "P1"},
			},
			want: map[string]string{
				"db1\x00p1": "p1",
				"db2\x00p1": "P1",
			},
		},
		{
			name: "no rows yields empty map",
			rows: nil,
			want: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, client.CanonicalRoutineMap(tt.rows))
		})
	}
}

func TestCanonicalizeRoutineNames(t *testing.T) {
	tests := []struct {
		name      string
		grants    []*client.RoutineGrant
		rows      []client.RoutineSchemaRow
		wantNames []string
	}{
		{
			name: "grant key case matches the synced resource",
			grants: []*client.RoutineGrant{
				{Database: "db1", RoutineName: "P1", Privs: "Execute"},
			},
			rows:      []client.RoutineSchemaRow{{Schema: "db1", Name: "p1"}},
			wantNames: []string{"p1"},
		},
		{
			name: "schema case mismatch also canonicalizes",
			grants: []*client.RoutineGrant{
				{Database: "DB1", RoutineName: "p1", Privs: "Execute"},
			},
			rows:      []client.RoutineSchemaRow{{Schema: "db1", Name: "P1"}},
			wantNames: []string{"P1"},
		},
		{
			name: "orphaned row for a dropped routine keeps procs_priv case",
			grants: []*client.RoutineGrant{
				{Database: "db1", RoutineName: "GONE", Privs: "Execute"},
			},
			rows:      []client.RoutineSchemaRow{{Schema: "db1", Name: "p1"}},
			wantNames: []string{"GONE"},
		},
		{
			name:      "no grants is a no-op",
			grants:    nil,
			rows:      []client.RoutineSchemaRow{{Schema: "db1", Name: "p1"}},
			wantNames: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			canonical := client.CanonicalRoutineMap(tt.rows)
			client.CanonicalizeRoutineNames(tt.grants, canonical)
			var got []string
			for _, g := range tt.grants {
				got = append(got, g.RoutineName)
			}
			require.Equal(t, tt.wantNames, got)
		})
	}
}

func TestRoutinePrivilege(t *testing.T) {
	tests := []struct {
		name         string
		rawPrivilege string
		want         string
		wantErr      string
	}{
		{
			name:         "execute",
			rawPrivilege: "execute",
			want:         "EXECUTE",
		},
		{
			name:         "alter_routine",
			rawPrivilege: "alter_routine",
			want:         "ALTER ROUTINE",
		},
		{
			name:         "grant",
			rawPrivilege: "grant",
			want:         "GRANT OPTION",
		},
		{
			name:         "uppercase GRANT still maps (helper lowercases)",
			rawPrivilege: "GRANT",
			want:         "GRANT OPTION",
		},
		{
			name:         "bogus",
			rawPrivilege: "bogus",
			wantErr:      "unsupported privilege for routine: bogus",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := routinePrivilege(tt.rawPrivilege)
			if tt.wantErr != "" {
				require.EqualError(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestRoutinePrincipalAllowed(t *testing.T) {
	tests := []struct {
		name         string
		resourceType string
		isVersion8   bool
		want         bool
	}{
		{
			name:         "user on 5.x",
			resourceType: "user",
			isVersion8:   false,
			want:         true,
		},
		{
			name:         "user on 8.x",
			resourceType: "user",
			isVersion8:   true,
			want:         true,
		},
		{
			name:         "role on 5.x rejected",
			resourceType: "role",
			isVersion8:   false,
			want:         false,
		},
		{
			name:         "role on 8.x allowed",
			resourceType: "role",
			isVersion8:   true,
			want:         true,
		},
		{
			name:         "database never allowed",
			resourceType: "database",
			isVersion8:   true,
			want:         false,
		},
		{
			name:         "empty resource type never allowed",
			resourceType: "",
			isVersion8:   true,
			want:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, routinePrincipalAllowed(tt.resourceType, tt.isVersion8))
		})
	}
}
