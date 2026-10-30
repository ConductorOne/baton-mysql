package connector

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestCatalogExhaustiveness(t *testing.T) {
	for rtID, tmpls := range entitlementsByResourceType {
		for _, tmpl := range tmpls {
			id := tmpl.ID
			t.Run(rtID+"/"+id, func(t *testing.T) {
				switch rtID {
				case resourceTypeServer.Id:
					base, _ := trimWithGrantSuffix(id)
					if base == "proxy" {
						_, _, err := resolveServerPrivilege("entitlement:" + id + ":server:x")
						require.Error(t, err)
						require.Equal(t, codes.Unimplemented, status.Code(err))
						return
					}
					name, withGrant, err := resolveServerPrivilege("entitlement:" + id + ":server:x")
					require.NoError(t, err)
					require.NotEmpty(t, name)
					require.Equal(t, strings.HasSuffix(id, "_with_grant"), withGrant)
				case resourceTypeDatabase.Id:
					base, _ := trimWithGrantSuffix(id)
					name, withGrant, err := sqlPrivilegeName(id)
					require.NoError(t, err)
					require.False(t, withGrant)
					if base == "grant" {
						require.Equal(t, "GRANT OPTION", name)
					} else {
						require.Equal(t, strings.ToUpper(strings.ReplaceAll(base, "_", " ")), name)
					}
				case resourceTypeTable.Id:
					name, withGrant, err := sqlPrivilegeName(id)
					require.NoError(t, err)
					require.False(t, withGrant)
					require.NotEmpty(t, name)
				case resourceTypeColumn.Id:
					base, _ := trimWithGrantSuffix(id)
					if base == "grant" {
						require.Equal(t, "grant", id)
					} else {
						require.NotContains(t, id, "_")
					}
				case resourceTypeRoutine.Id:
					require.Contains(t, []string{"execute", "alter_routine", "grant"}, id)
					_, err := routinePrivilege(id)
					require.NoError(t, err)
				case resourceTypeUser.Id, resourceTypeRole.Id:
					// user/role-scope slugs are owned by roleSyncer, not the shared mapper.
					require.Contains(t, []string{
						proxyPriv,
						proxyWithGrantPriv,
						roleAssignmentPriv,
						roleAssignmentWithGrantPriv,
					}, id)
				}
			})
		}
	}

	// Fail-closed: user/role-scope slugs never route through the shared mapper.
	_, _, err := sqlPrivilegeName("role_assignment")
	require.Error(t, err)
	_, _, err = sqlPrivilegeName("proxy")
	require.Error(t, err)
}
