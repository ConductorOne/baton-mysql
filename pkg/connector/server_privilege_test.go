package connector

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// catalogKeyOf extracts the catalog key (the middle segment) of an entitlement
// ID of the form "entitlement:<key>:<resource>".
func catalogKeyOf(id string) string {
	return strings.SplitN(id, ":", 3)[1]
}

// keysOf returns the keys of m, for set-equality assertions against the live catalog.
func keysOf[V any](m map[string]V) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	return ks
}

// TestServerPrivilege asserts, exhaustively over the live server catalog, the
// privilege keyword and grant-option flag serverPrivilege produces. The
// expected table is compared for set equality with the live catalog keys, so a
// future catalog addition (or a new _with_grant variant) fails here instead of
// shipping bad SQL.
func TestServerPrivilege(t *testing.T) {
	type want struct {
		keyword   string
		withGrant bool
	}

	expected := make(map[string]want)

	// Reserved keys: byte-identical pre-fix emission (I6).
	expected["proxy"] = want{"PROXY", false}
	expected["proxy_with_grant"] = want{"PROXY_WITH_GRANT WITH GRANT OPTION", false}

	// Multi-word static privileges, space-separated.
	for key, keyword := range staticServerPrivileges {
		expected[key] = want{keyword, false}
	}

	// Single-word static privileges: ToUpper.
	for _, key := range []string{
		"alter", "create", "delete", "drop", "event", "execute", "file",
		"index", "insert", "process", "references", "reload", "select",
		"shutdown", "super", "trigger", "update",
	} {
		expected[key] = want{strings.ToUpper(key), false}
	}

	// Dynamic privileges keep their underscore form; each has a _with_grant variant.
	for _, key := range []string{
		"application_password_admin", "audit_abort_exempt", "audit_admin",
		"authentication_policy_admin", "backup_admin", "binlog_admin",
		"binlog_encryption_admin", "clone_admin", "connection_admin",
		"encryption_key_admin", "firewall_admin", "firewall_exempt",
		"firewall_user", "flush_optimizer_costs", "flush_status", "flush_tables",
		"flush_user_resources", "group_replication_admin", "group_replication_stream",
		"innodb_redo_log_enable", "innodb_redo_log_archive", "nbd_stored_user",
		"passwordless_user_admin", "persist_ro_variables_admin", "replication_applier",
		"replication_slave_admin", "resource_group_admin", "resource_group_user",
		"role_admin", "sensitive_variables_observer", "service_connection_admin",
		"session_variables_admin", "set_user_id", "show_routine", "skip_query_rewrite",
		"system_user", "system_variables_admin", "table_encryption_admin",
		"version_token_admin", "xa_recover_admin",
	} {
		expected[key] = want{strings.ToUpper(key), false}
		expected[key+"_with_grant"] = want{strings.ToUpper(key), true}
	}

	liveIDs := make([]string, 0, len(entitlementsByResourceType[resourceTypeServer.Id]))
	for _, tmpl := range entitlementsByResourceType[resourceTypeServer.Id] {
		liveIDs = append(liveIDs, "entitlement:"+tmpl.ID+":server:x")
	}

	liveKeys := make([]string, 0, len(liveIDs))
	for _, id := range liveIDs {
		liveKeys = append(liveKeys, catalogKeyOf(id))
	}
	require.ElementsMatch(t, liveKeys, keysOf(expected))

	for _, id := range liveIDs {
		key := catalogKeyOf(id)
		kw, wg, err := serverPrivilege(id)
		require.NoError(t, err)
		require.Equal(t, expected[key].keyword, kw, "keyword for %s", key)
		require.Equal(t, expected[key].withGrant, wg, "withGrant for %s", key)
	}

	for _, bad := range []string{"garbage", "entitlement::server:x", "a:b"} {
		_, _, err := serverPrivilege(bad)
		require.Error(t, err, "expected error for %q", bad)
	}
}
