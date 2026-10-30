package connector

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestTrimWithGrantSuffix(t *testing.T) {
	tests := []struct {
		slug      string
		wantBase  string
		wantGrant bool
	}{
		{slug: "show_databases", wantBase: "show_databases", wantGrant: false},
		{slug: "backup_admin_with_grant", wantBase: "backup_admin", wantGrant: true},
		{slug: "backup_admin_WITH_GRANT", wantBase: "backup_admin", wantGrant: true},
		{slug: "BACKUP_ADMIN_with_grant", wantBase: "backup_admin", wantGrant: true},
		{slug: "grant", wantBase: "grant", wantGrant: false},
		{slug: "", wantBase: "", wantGrant: false},
	}
	for _, tt := range tests {
		t.Run(tt.slug, func(t *testing.T) {
			base, withGrant := trimWithGrantSuffix(tt.slug)
			require.Equal(t, tt.wantBase, base)
			require.Equal(t, tt.wantGrant, withGrant)
		})
	}
}

func TestSqlPrivilegeName(t *testing.T) {
	for _, tt := range []struct {
		slug string
		want string
	}{
		{slug: "alter_routine", want: "ALTER ROUTINE"},
		{slug: "create_role", want: "CREATE ROLE"},
		{slug: "create_routine", want: "CREATE ROUTINE"},
		{slug: "create_tablespace", want: "CREATE TABLESPACE"},
		{slug: "create_temporary_tables", want: "CREATE TEMPORARY TABLES"},
		{slug: "create_user", want: "CREATE USER"},
		{slug: "create_view", want: "CREATE VIEW"},
		{slug: "drop_role", want: "DROP ROLE"},
		{slug: "lock_tables", want: "LOCK TABLES"},
		{slug: "replication_client", want: "REPLICATION CLIENT"},
		{slug: "replication_slave", want: "REPLICATION SLAVE"},
		{slug: "show_databases", want: "SHOW DATABASES"},
		{slug: "show_view", want: "SHOW VIEW"},
		{slug: "grant", want: "GRANT OPTION"},
		// Both spellings of the NDB stored-user privilege map to MySQL's
		// NDB_STORED_USER: the transposed catalog slug and the DB's own
		// lowercased spelling sync emits on revoke.
		{slug: "nbd_stored_user", want: "NDB_STORED_USER"},
		{slug: "ndb_stored_user", want: "NDB_STORED_USER"},
	} {
		t.Run(tt.slug, func(t *testing.T) {
			name, withGrant, err := sqlPrivilegeName(tt.slug)
			require.NoError(t, err)
			require.False(t, withGrant)
			require.Equal(t, tt.want, name)
		})
	}

	for _, slug := range []string{
		"alter", "create", "delete", "drop", "event", "execute", "file", "index",
		"insert", "process", "references", "reload", "select", "shutdown", "super",
		"trigger", "update",
	} {
		t.Run(slug, func(t *testing.T) {
			name, withGrant, err := sqlPrivilegeName(slug)
			require.NoError(t, err)
			require.False(t, withGrant)
			require.Equal(t, strings.ToUpper(slug), name)
			require.NotContains(t, name, " ")
			require.NotContains(t, name, "_")
		})
	}

	for slug := range dynamicPrivileges {
		want := strings.ToUpper(slug)
		if sql, ok := privilegeSpellingOverrides[slug]; ok {
			want = sql
		}
		t.Run(slug, func(t *testing.T) {
			name, withGrant, err := sqlPrivilegeName(slug)
			require.NoError(t, err)
			require.False(t, withGrant)
			require.Equal(t, want, name)
		})
	}

	sqlName, withGrant, err := sqlPrivilegeName("backup_admin")
	require.NoError(t, err)
	require.False(t, withGrant)
	require.Equal(t, "BACKUP_ADMIN", sqlName)

	sqlName, withGrant, err = sqlPrivilegeName("show_routine")
	require.NoError(t, err)
	require.False(t, withGrant)
	require.Equal(t, "SHOW_ROUTINE", sqlName)

	require.Len(t, dynamicPrivileges, 40)

	_, _, err = sqlPrivilegeName("not_a_real_priv")
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	require.Contains(t, err.Error(), `baton-mysql: unrecognized privilege "not_a_real_priv"`)
}

func TestSqlPrivilegeNameWithGrant(t *testing.T) {
	tests := []struct {
		slug      string
		wantSQL   string
		wantGrant bool
	}{
		{slug: "show_databases_with_grant", wantSQL: "SHOW DATABASES", wantGrant: true},
		{slug: "backup_admin_with_grant", wantSQL: "BACKUP_ADMIN", wantGrant: true},
		{slug: "SELECT_with_GRANT", wantSQL: "SELECT", wantGrant: true},
		{slug: "nbd_stored_user_with_grant", wantSQL: "NDB_STORED_USER", wantGrant: true},
	}
	for _, tt := range tests {
		t.Run(tt.slug, func(t *testing.T) {
			name, withGrant, err := sqlPrivilegeName(tt.slug)
			require.NoError(t, err)
			require.Equal(t, tt.wantSQL, name)
			require.Equal(t, tt.wantGrant, withGrant)
		})
	}
}

func TestSqlPrivilegeNameCharset(t *testing.T) {
	// Underscore-free garbage (no underscore, not in the static map, not a
	// known dynamic privilege) must fail closed like underscore-bearing garbage.
	for _, slug := range []string{"garbage!!", "not-real", "GRANT-OPTION", "grant option"} {
		_, _, err := sqlPrivilegeName(slug)
		require.Error(t, err, slug)
		require.Equal(t, codes.InvalidArgument, status.Code(err))
	}
	// Underscore-bearing garbage still fails closed.
	_, _, err := sqlPrivilegeName("not_a_real_priv")
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestResolveServerPrivilege(t *testing.T) {
	name, withGrant, err := resolveServerPrivilege("entitlement:show_databases:server:srv1")
	require.NoError(t, err)
	require.False(t, withGrant)
	require.Equal(t, "SHOW DATABASES", name)

	name, withGrant, err = resolveServerPrivilege("entitlement:backup_admin_with_grant:server:srv1")
	require.NoError(t, err)
	require.True(t, withGrant)
	require.Equal(t, "BACKUP_ADMIN", name)

	// The transposed catalog slug with _with_grant must keep its withGrant
	// flag (the server grant path composes the trailing clause) while the
	// mapper corrects the SQL spelling.
	name, withGrant, err = resolveServerPrivilege("entitlement:nbd_stored_user_with_grant:server:srv1")
	require.NoError(t, err)
	require.True(t, withGrant)
	require.Equal(t, "NDB_STORED_USER", name)

	_, _, err = resolveServerPrivilege("entitlement:proxy:server:srv1")
	require.Error(t, err)
	require.Equal(t, codes.Unimplemented, status.Code(err))
	require.Contains(t, err.Error(), "baton-mysql: proxy grants at server scope are not supported by MySQL grammar")

	_, _, err = resolveServerPrivilege("entitlement:proxy_with_grant:server:srv1")
	require.Error(t, err)
	require.Equal(t, codes.Unimplemented, status.Code(err))
	require.Contains(t, err.Error(), "baton-mysql: proxy grants at server scope are not supported by MySQL grammar")

	_, _, err = resolveServerPrivilege("entitlement:show_databases")
	require.Error(t, err)
	require.Contains(t, err.Error(), fmt.Sprintf("invalid entitlement ID: %s", "entitlement:show_databases"))

	_, _, err = resolveServerPrivilege("entitlement:not_a_real_priv:server:x")
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestGrantPrivilegeStringRejectsWithGrant(t *testing.T) {
	// WITH GRANT OPTION at database/table scope builds invalid MySQL grammar
	// (the clause must trail the TO clause); no catalog *_with_grant
	// entitlement exists at these scopes, so fail closed instead.
	_, err := grantPrivilegeString("SELECT", true)
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	require.Contains(t, err.Error(), "WITH GRANT OPTION is not supported at database or table scope")

	sql, err := grantPrivilegeString("SELECT", false)
	require.NoError(t, err)
	require.Equal(t, "SELECT", sql)
}

func TestDatabaseTableSyncerWithGrantRejected(t *testing.T) {
	ctx := context.Background()
	user := userPrincipal("user:alice@localhost")

	db := newDatabaseSyncer(nil, map[string]struct{}{})
	_, err := db.Grant(ctx, user, entitlement("entitlement:select_with_grant:database:db1"))
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	require.Contains(t, err.Error(), "WITH GRANT OPTION is not supported at database or table scope")

	ts := newTableSyncer(nil, map[string]struct{}{})
	_, err = ts.Grant(ctx, user, entitlement("entitlement:select_with_grant:table:db1.tbl1"))
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	require.Contains(t, err.Error(), "WITH GRANT OPTION is not supported at database or table scope")
}
