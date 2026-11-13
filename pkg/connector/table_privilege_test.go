package connector

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestTablePrivilege asserts, exhaustively over the live table catalog, the
// privilege keyword tablePrivilege produces. Set equality with the live keys
// makes a future catalog addition fail here instead of shipping bad SQL.
func TestTablePrivilege(t *testing.T) {
	expected := map[string]string{
		"create_view": "CREATE VIEW",
		"show_view":   "SHOW VIEW",
		"grant":       "GRANT OPTION",
		"alter":       "ALTER",
		"create":      "CREATE",
		"delete":      "DELETE",
		"drop":        "DROP",
		"index":       "INDEX",
		"insert":      "INSERT",
		"references":  "REFERENCES",
		"select":      "SELECT",
		"trigger":     "TRIGGER",
		"update":      "UPDATE",
	}

	live := make([]string, 0, len(entitlementsByResourceType[resourceTypeTable.Id]))
	for _, tmpl := range entitlementsByResourceType[resourceTypeTable.Id] {
		live = append(live, tmpl.ID)
	}
	require.ElementsMatch(t, live, keysOf(expected))

	for key, keyword := range expected {
		require.Equal(t, keyword, tablePrivilege(key), "keyword for %s", key)
	}
}
