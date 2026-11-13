package connector

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestDatabasePrivilege asserts, exhaustively over the live database catalog,
// the privilege keyword extractDatabasePrivilegeAndDb produces. Only the grant
// key differs from the pre-fix blanket underscore-to-space conversion; the
// expected values below pin that byte-for-byte.
func TestDatabasePrivilege(t *testing.T) {
	expected := map[string]string{
		"grant":                   "GRANT OPTION",
		"alter":                   "ALTER",
		"alter_routine":           "ALTER ROUTINE",
		"create":                  "CREATE",
		"create_routine":          "CREATE ROUTINE",
		"create_temporary_tables": "CREATE TEMPORARY TABLES",
		"create_view":             "CREATE VIEW",
		"delete":                  "DELETE",
		"drop":                    "DROP",
		"event":                   "EVENT",
		"execute":                 "EXECUTE",
		"index":                   "INDEX",
		"insert":                  "INSERT",
		"lock_tables":             "LOCK TABLES",
		"references":              "REFERENCES",
		"select":                  "SELECT",
		"show_view":               "SHOW VIEW",
		"trigger":                 "TRIGGER",
		"update":                  "UPDATE",
	}

	live := make([]string, 0, len(entitlementsByResourceType[resourceTypeDatabase.Id]))
	for _, tmpl := range entitlementsByResourceType[resourceTypeDatabase.Id] {
		live = append(live, tmpl.ID)
	}
	require.ElementsMatch(t, live, keysOf(expected))

	for _, key := range live {
		priv, db := extractDatabasePrivilegeAndDb("entitlement:" + key + ":db:testdb")
		require.Equal(t, "testdb", db, "database for %s", key)
		require.Equal(t, expected[key], priv, "keyword for %s", key)
	}
}
