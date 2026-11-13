package client

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// privilegeSlotOf returns the text between the leading verb and " ON *.* ",
// i.e. the privilege keyword slot of a server-scope statement.
func privilegeSlotOf(t *testing.T, stmt, verb string) string {
	t.Helper()
	start := len(verb) + 1 // verb + space
	end := strings.Index(stmt, " ON *.* ")
	require.Greater(t, end, start, "statement %q has no privilege slot", stmt)
	return stmt[start:end]
}

func TestServerGrantStatement(t *testing.T) {
	tests := []struct {
		name      string
		user      string
		privilege string
		withGrant bool
		want      string
	}{
		{
			name:      "no grant option",
			user:      "alice@localhost",
			privilege: "CREATE USER",
			withGrant: false,
			want:      "GRANT CREATE USER ON *.* TO 'alice'@'localhost'",
		},
		{
			name:      "with grant option",
			user:      "alice@localhost",
			privilege: "BACKUP_ADMIN",
			withGrant: true,
			want:      "GRANT BACKUP_ADMIN ON *.* TO 'alice'@'localhost' WITH GRANT OPTION",
		},
		{
			name:      "grant option privilege",
			user:      "alice@localhost",
			privilege: "GRANT OPTION",
			withGrant: false,
			want:      "GRANT GRANT OPTION ON *.* TO 'alice'@'localhost'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			quoted, err := quoteAccounts(tt.user)
			require.NoError(t, err)

			got := serverGrantStatement(quoted, tt.privilege, tt.withGrant)
			require.Equal(t, tt.want, got)

			slot := privilegeSlotOf(t, got, "GRANT")
			require.NotContains(t, slot, "_with_grant")
			require.NotContains(t, slot, "WITH GRANT OPTION")

			if tt.withGrant {
				require.True(t, strings.HasSuffix(got, " WITH GRANT OPTION"), "clause must follow the account: %q", got)
			} else {
				require.NotContains(t, got, " WITH GRANT OPTION")
			}
		})
	}
}

func TestServerRevokeStatement(t *testing.T) {
	tests := []struct {
		name      string
		user      string
		privilege string
		want      string
	}{
		{
			name:      "multi-word",
			user:      "alice@localhost",
			privilege: "CREATE USER",
			want:      "REVOKE CREATE USER ON *.* FROM 'alice'@'localhost'",
		},
		{
			name:      "grant option privilege",
			user:      "alice@localhost",
			privilege: "GRANT OPTION",
			want:      "REVOKE GRANT OPTION ON *.* FROM 'alice'@'localhost'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			quoted, err := quoteAccounts(tt.user)
			require.NoError(t, err)

			got := serverRevokeStatement(quoted, tt.privilege)
			require.Equal(t, tt.want, got)

			slot := privilegeSlotOf(t, got, "REVOKE")
			require.NotContains(t, slot, "_with_grant")
			require.NotContains(t, slot, " WITH GRANT OPTION")
		})
	}
}
