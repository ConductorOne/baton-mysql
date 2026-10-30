package connector

import (
	"fmt"
	"regexp"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Multi-word static privilege names as MySQL expects them in GRANT/REVOKE.
const grantOptionSlug = "grant"

var staticPrivilegeSQL = map[string]string{
	"alter_routine":           "ALTER ROUTINE",
	"create_role":             "CREATE ROLE",
	"create_routine":          "CREATE ROUTINE",
	"create_tablespace":       "CREATE TABLESPACE",
	"create_temporary_tables": "CREATE TEMPORARY TABLES",
	"create_user":             "CREATE USER",
	"create_view":             "CREATE VIEW",
	"drop_role":               "DROP ROLE",
	"lock_tables":             "LOCK TABLES",
	"replication_client":      "REPLICATION CLIENT",
	"replication_slave":       "REPLICATION SLAVE",
	"show_databases":          "SHOW DATABASES",
	"show_view":               "SHOW VIEW",
	grantOptionSlug:           "GRANT OPTION",
}

// MySQL dynamic privileges are registered with underscores and are not translated.
var dynamicPrivileges = map[string]struct{}{
	"application_password_admin":   {},
	"audit_abort_exempt":           {},
	"audit_admin":                  {},
	"authentication_policy_admin":  {},
	"backup_admin":                 {},
	"binlog_admin":                 {},
	"binlog_encryption_admin":      {},
	"clone_admin":                  {},
	"connection_admin":             {},
	"encryption_key_admin":         {},
	"firewall_admin":               {},
	"firewall_exempt":              {},
	"firewall_user":                {},
	"flush_optimizer_costs":        {},
	"flush_status":                 {},
	"flush_tables":                 {},
	"flush_user_resources":         {},
	"group_replication_admin":      {},
	"group_replication_stream":     {},
	"innodb_redo_log_archive":      {},
	"innodb_redo_log_enable":       {},
	"nbd_stored_user":              {},
	"passwordless_user_admin":      {},
	"persist_ro_variables_admin":   {},
	"replication_applier":          {},
	"replication_slave_admin":      {},
	"resource_group_admin":         {},
	"resource_group_user":          {},
	"role_admin":                   {},
	"sensitive_variables_observer": {},
	"service_connection_admin":     {},
	"session_variables_admin":      {},
	"set_user_id":                  {},
	"show_routine":                 {},
	"skip_query_rewrite":           {},
	"system_user":                  {},
	"system_variables_admin":       {},
	"table_encryption_admin":       {},
	"version_token_admin":          {},
	"xa_recover_admin":             {},
}

func trimWithGrantSuffix(slug string) (string, bool) {
	lowered := strings.ToLower(slug)
	withGrant := strings.HasSuffix(lowered, "_with_grant")
	if withGrant {
		base := lowered[:len(lowered)-len("_with_grant")]
		return base, withGrant
	}
	return lowered, withGrant
}

// grantPrivilegeString fails closed when withGrant is set at database/table
// scope: MySQL grammar requires WITH GRANT OPTION to trail the TO clause, which
// GrantDatabasePrivilege/GrantTablePrivilege do not support, and no catalog
// *_with_grant entitlement exists at these scopes (includeWithGrant is
// server-only). Server-scope WITH GRANT OPTION is handled by client.GrantServerPrivilege.
func grantPrivilegeString(sqlName string, withGrant bool) (string, error) {
	if withGrant {
		return "", status.Errorf(codes.InvalidArgument, "baton-mysql: WITH GRANT OPTION is not supported at database or table scope")
	}
	return sqlName, nil
}

// baseSlugRe validates the base slug charset: slugs are lower-case snake per
// the entitlement catalog, so fail closed on anything else (e.g. underscore-free garbage).
var baseSlugRe = regexp.MustCompile(`^[a-z_]+$`)

func sqlPrivilegeName(slug string) (string, bool, error) {
	base, withGrant := trimWithGrantSuffix(strings.ToLower(slug))

	if !baseSlugRe.MatchString(base) {
		return "", false, status.Errorf(codes.InvalidArgument, "baton-mysql: unrecognized privilege %q", slug)
	}

	if sql, ok := staticPrivilegeSQL[base]; ok {
		return sql, withGrant, nil
	}
	switch base {
	case proxyPriv, proxyWithGrantPriv, roleAssignmentPriv, roleAssignmentWithGrantPriv:
		return "", false, status.Errorf(codes.InvalidArgument, "baton-mysql: unrecognized privilege %q", slug)
	}
	if strings.Contains(base, "_") {
		if _, ok := dynamicPrivileges[base]; ok {
			return strings.ToUpper(base), withGrant, nil
		}
		return "", false, status.Errorf(codes.InvalidArgument, "baton-mysql: unrecognized privilege %q", slug)
	}
	return strings.ToUpper(base), withGrant, nil
}

func resolveServerPrivilege(entitlementID string) (string, bool, error) {
	parts := strings.Split(entitlementID, ":")
	if len(parts) < 3 {
		return "", false, fmt.Errorf("invalid entitlement ID: %s", entitlementID)
	}

	if base, _ := trimWithGrantSuffix(parts[1]); base == proxyPriv {
		return "", false, status.Errorf(codes.Unimplemented, "baton-mysql: proxy grants at server scope are not supported by MySQL grammar")
	}

	return sqlPrivilegeName(parts[1])
}
