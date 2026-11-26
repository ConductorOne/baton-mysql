package client

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestMatchPrivilege(t *testing.T) {
	registered := []string{
		"Select",
		"Show databases",
		"Create view",
		"Create temporary tables",
		"Grant option",
		"BACKUP_ADMIN",
		"NDB_STORED_USER",
		"Proxy",
	}

	tests := []struct {
		slug      string
		name      string
		withGrant bool
		code      codes.Code
	}{
		{slug: "select", name: "SELECT"},
		{slug: "show_databases", name: "SHOW DATABASES"},
		{slug: "create_view", name: "CREATE VIEW"},
		{slug: "create_temporary_tables", name: "CREATE TEMPORARY TABLES"},
		{slug: grantPrivilege, name: "GRANT OPTION"},
		{slug: "backup_admin", name: "BACKUP_ADMIN"},
		{slug: "backup_admin_with_grant", name: "BACKUP_ADMIN", withGrant: true},
		// The published slug is misspelled and cannot be renamed without re-keying
		// every grant that holds it, so the alias resolves it here instead.
		{slug: "nbd_stored_user", name: "NDB_STORED_USER"},
		{slug: "nbd_stored_user_with_grant", name: "NDB_STORED_USER", withGrant: true},
		{slug: proxyPrivilege, code: codes.InvalidArgument},
		{slug: "show_databases_typo", code: codes.InvalidArgument},
	}

	for _, tt := range tests {
		t.Run(tt.slug, func(t *testing.T) {
			name, withGrant, err := MatchPrivilege(tt.slug, registered)
			if tt.code != codes.OK {
				require.Error(t, err)
				require.Equal(t, tt.code, status.Code(err))
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.name, name)
			require.Equal(t, tt.withGrant, withGrant)
		})
	}
}
