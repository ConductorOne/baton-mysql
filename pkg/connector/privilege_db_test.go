package connector

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/require"

	"github.com/conductorone/baton-mysql/pkg/client"
)

// TestPrivilegeSpellingAgainstMySQL exercises the built GRANT/REVOKE statements
// against a real MySQL 8 server. It first reproduces the pre-fix failure (the
// old statements are rejected) and then verifies every live catalog privilege at
// every provisionable scope executes cleanly through the real client.
//
// The test is gated on BATON_CONNECTION_STRING (not an unconditional skip) and
// fails loudly if the DSN is set but unreachable. It skips in CI (no CI go-test
// job sets the DSN), so this evidence is a local mysql:8 run.
//
// The reserved server-scope keys proxy and proxy_with_grant are deliberately
// not swept, because their emission stays byte-identical to pre-fix and is not
// a valid GRANT <keyword> ON *.* form.
func TestPrivilegeSpellingAgainstMySQL(t *testing.T) {
	dsn := os.Getenv("BATON_CONNECTION_STRING")
	if dsn == "" {
		t.Skip("BATON_CONNECTION_STRING not set; skipping MySQL-backed privilege-spelling regression")
	}

	ctx := context.Background()
	c, err := client.New(ctx, dsn)
	require.NoError(t, err)

	const (
		probeUser  = "baton_probe@%"
		probeTable = "testdb.baton_probe_table"
		probeDB    = "testdb"
	)

	_, err = c.ExecContext(ctx, "CREATE USER IF NOT EXISTS 'baton_probe'@'%' IDENTIFIED BY 'baton_probe_pw'")
	require.NoError(t, err)
	_, err = c.ExecContext(ctx, "CREATE TABLE IF NOT EXISTS `testdb`.`baton_probe_table` (id INT)")
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = c.ExecContext(ctx, "DROP TABLE IF EXISTS `testdb`.`baton_probe_table`")
		_, _ = c.ExecContext(ctx, "DROP USER IF EXISTS 'baton_probe'@'%'")
	})

	// --- AC5: reproduce the pre-fix (rejected) statements. ---
	rejected := []string{
		"GRANT CREATE_USER ON *.* TO 'baton_probe'@'%'",
		"GRANT SHOW_DATABASES ON *.* TO 'baton_probe'@'%'",
		"GRANT CREATE_VIEW ON `testdb`.`baton_probe_table` TO 'baton_probe'@'%'",
		"GRANT BACKUP_ADMIN_WITH_GRANT WITH GRANT OPTION ON *.* TO 'baton_probe'@'%'",
	}
	for _, stmt := range rejected {
		_, err := c.ExecContext(ctx, stmt)
		require.Error(t, err, "pre-fix statement should be rejected: %s", stmt)
		var mysqlErr *mysql.MySQLError
		if errors.As(err, &mysqlErr) {
			require.Contains(t, []uint16{1064, 1149, 3619}, mysqlErr.Number,
				"unexpected error number for %s: %s", stmt, err)
		}
	}

	// --- AC4: verify the fixed statements execute cleanly. ---
	// allowed permits only access-denied / privilege-required outcomes; a
	// syntax or grant-option error (1064/1149/3619) is always a failure.
	allowed := func(err error) bool {
		if err == nil {
			return true
		}
		var mysqlErr *mysql.MySQLError
		return errors.As(err, &mysqlErr) && (mysqlErr.Number == 1044 || mysqlErr.Number == 1045 || mysqlErr.Number == 1227)
	}
	fail := func(key, keyword string, err error) {
		var mysqlErr *mysql.MySQLError
		num := ""
		if errors.As(err, &mysqlErr) {
			num = mysqlErr.Error()
		}
		t.Fatalf("privilege %q (keyword %q) failed: %v %s", key, keyword, err, num)
	}

	// pluginDependentServerPrivileges are dynamic privileges MySQL registers only
	// when the matching plugin or component is loaded (Enterprise Firewall, audit
	// log, query rewrite, version tokens, NDB cluster). A stock mysql:8 rejects
	// them as unknown, so the sweep skips them: it verifies keyword spelling, not
	// plugin presence. AUDIT_ADMIN is deliberately absent: the server core
	// registers it unconditionally in dynamic_privilege_init(), so GRANT
	// AUDIT_ADMIN succeeds even without the audit_log plugin and is swept.
	pluginDependentServerPrivileges := map[string]struct{}{
		"audit_abort_exempt":  {},
		"firewall_admin":      {},
		"firewall_exempt":     {},
		"firewall_user":       {},
		"nbd_stored_user":     {},
		"skip_query_rewrite":  {},
		"version_token_admin": {},
	}

	// versionDependentServerPrivileges are dynamic privileges a later MySQL 8.x
	// release removed but the catalog still emits for 8.0. SET_USER_ID was
	// deprecated in 8.2.0 and removed in 8.4.0 (replaced by SET_ANY_DEFINER), so
	// a stock mysql:8 (currently 8.4) rejects it as unknown; the sweep skips it
	// to verify keyword spelling, not server version.
	versionDependentServerPrivileges := map[string]struct{}{
		"set_user_id": {},
	}

	// server scope
	for _, tmpl := range entitlementsByResourceType[resourceTypeServer.Id] {
		if tmpl.ID == "proxy" || tmpl.ID == "proxy_with_grant" {
			continue // reserved key; not a valid GRANT <keyword> ON *.* form
		}
		key := strings.TrimSuffix(tmpl.ID, "_with_grant")
		if _, skip := pluginDependentServerPrivileges[key]; skip {
			continue // privilege exists only once its plugin/component is loaded
		}
		if _, skip := versionDependentServerPrivileges[key]; skip {
			continue // privilege removed in a later MySQL 8.x release
		}
		keyword, withGrant, err := serverPrivilege("entitlement:" + tmpl.ID + ":server:x")
		require.NoError(t, err)
		if err := c.GrantServerPrivilege(ctx, probeUser, keyword, withGrant); !allowed(err) {
			fail(tmpl.ID, keyword, err)
		}
		if err := c.RevokeServerPrivilege(ctx, probeUser, keyword); !allowed(err) {
			fail(tmpl.ID, keyword, err)
		}
	}

	// table scope
	for _, tmpl := range entitlementsByResourceType[resourceTypeTable.Id] {
		keyword := tablePrivilege(tmpl.ID)
		if err := c.GrantTablePrivilege(ctx, probeTable, probeUser, keyword); !allowed(err) {
			fail(tmpl.ID, keyword, err)
		}
		if err := c.RevokeTablePrivilege(ctx, probeTable, probeUser, keyword); !allowed(err) {
			fail(tmpl.ID, keyword, err)
		}
	}

	// database scope
	for _, tmpl := range entitlementsByResourceType[resourceTypeDatabase.Id] {
		keyword, _ := extractDatabasePrivilegeAndDb("entitlement:" + tmpl.ID + ":db:testdb")
		if err := c.GrantDatabasePrivilege(ctx, probeDB, probeUser, keyword); !allowed(err) {
			fail(tmpl.ID, keyword, err)
		}
		if err := c.RevokeDatabasePrivilege(ctx, probeDB, probeUser, keyword); !allowed(err) {
			fail(tmpl.ID, keyword, err)
		}
	}
}
