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

func TestRoutineHeldPrivs(t *testing.T) {
	tests := []struct {
		name   string
		grants []*client.RoutineGrant
		want   []string
	}{
		{
			name:   "matching row decodes uppercased with spaces",
			grants: []*client.RoutineGrant{{Database: "db1", RoutineName: "p1", Privs: "Execute,Alter Routine,Grant"}},
			want:   []string{"ALTER ROUTINE", "EXECUTE"},
		},
		{
			name: "grant-only row yields empty",
			grants: []*client.RoutineGrant{
				{Database: "db1", RoutineName: "p1", Privs: "Grant"},
			},
			want: []string{},
		},
		{
			name: "non-matching schema or routine ignored",
			grants: []*client.RoutineGrant{
				{Database: "other", RoutineName: "p1", Privs: "Execute"},
				{Database: "db1", RoutineName: "other", Privs: "Execute"},
			},
			want: []string{},
		},
		{
			name: "collapsed rows union with dedupe",
			grants: []*client.RoutineGrant{
				{Database: "db1", RoutineName: "p1", Privs: "Execute"},
				{Database: "db1", RoutineName: "p1", Privs: "Execute,Grant"},
			},
			want: []string{"EXECUTE"},
		},
		{
			name: "sorted deterministically",
			grants: []*client.RoutineGrant{
				{Database: "db1", RoutineName: "p1", Privs: "Alter Routine,Execute"},
			},
			want: []string{"ALTER ROUTINE", "EXECUTE"},
		},
		{
			name:   "no rows yields empty",
			grants: nil,
			want:   []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, routineHeldPrivs(tt.grants, "db1", "p1"))
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
