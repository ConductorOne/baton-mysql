package connector

import (
	"context"
	"testing"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func userPrincipal(id string) *v2.Resource {
	return &v2.Resource{Id: &v2.ResourceId{ResourceType: resourceTypeUser.Id, Resource: id}}
}

func entitlement(id string) *v2.Entitlement {
	return &v2.Entitlement{Id: id}
}

func grant(id string, principal *v2.Resource) *v2.Grant {
	return &v2.Grant{
		Id: "grant:" + id,
		Entitlement: &v2.Entitlement{
			Id: id,
		},
		Principal: principal,
	}
}

func TestServerSyncerRejections(t *testing.T) {
	ctx := context.Background()
	s := newServerSyncer(nil)
	principal := userPrincipal("user:alice@localhost")

	for _, id := range []string{
		"entitlement:proxy:server:srv1",
		"entitlement:proxy_with_grant:server:srv1",
	} {
		t.Run("grant/"+id, func(t *testing.T) {
			_, err := s.Grant(ctx, principal, entitlement(id))
			require.Error(t, err)
			require.Equal(t, codes.Unimplemented, status.Code(err))
			require.Contains(t, err.Error(), "proxy grants at server scope are not supported")
		})
		t.Run("revoke/"+id, func(t *testing.T) {
			_, err := s.Revoke(ctx, grant(id, principal))
			require.Error(t, err)
			require.Equal(t, codes.Unimplemented, status.Code(err))
			require.Contains(t, err.Error(), "proxy grants at server scope are not supported")
		})
	}

	t.Run("unrecognized privilege", func(t *testing.T) {
		_, err := s.Grant(ctx, principal, entitlement("entitlement:not_a_real_priv:server:x"))
		require.Error(t, err)
		require.Equal(t, codes.InvalidArgument, status.Code(err))
		_, err = s.Revoke(ctx, grant("entitlement:not_a_real_priv:server:x", principal))
		require.Error(t, err)
		require.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("malformed entitlement ID", func(t *testing.T) {
		_, err := s.Grant(ctx, principal, entitlement("entitlement:select"))
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid entitlement ID")
		_, err = s.Revoke(ctx, grant("entitlement:select", principal))
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid entitlement ID")
	})
}

func TestColumnSyncerGuards(t *testing.T) {
	ctx := context.Background()
	c := newColumnSyncer(nil, map[string]struct{}{})
	principal := userPrincipal("user:alice@localhost")

	t.Run("grant malformed entitlement ID", func(t *testing.T) {
		_, err := c.Grant(ctx, principal, entitlement("entitlement:select"))
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid entitlement ID")
	})

	t.Run("revoke malformed column ID", func(t *testing.T) {
		_, err := c.Revoke(ctx, grant("entitlement:select:column:db.tbl", principal))
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid column ID format")
	})
}

func TestTableSyncerGuards(t *testing.T) {
	ctx := context.Background()
	ts := newTableSyncer(nil, map[string]struct{}{})
	user := userPrincipal("user:alice@localhost")

	t.Run("role principal rejected", func(t *testing.T) {
		role := &v2.Resource{Id: &v2.ResourceId{ResourceType: resourceTypeRole.Id, Resource: "role:app_reader@%"}}
		_, err := ts.Grant(ctx, role, entitlement("entitlement:select:table:db1.tbl1"))
		require.Error(t, err)
		require.Contains(t, err.Error(), "can only grant table permissions to users")
	})

	t.Run("malformed entitlement ID", func(t *testing.T) {
		_, err := ts.Grant(ctx, user, entitlement("entitlement:select:table"))
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid entitlement ID")
	})
}

func TestRoutinePrivilege(t *testing.T) {
	name, err := routinePrivilege("execute")
	require.NoError(t, err)
	require.Equal(t, "EXECUTE", name)

	name, err = routinePrivilege("alter_routine")
	require.NoError(t, err)
	require.Equal(t, "ALTER ROUTINE", name)

	name, err = routinePrivilege("grant")
	require.NoError(t, err)
	require.Equal(t, "GRANT OPTION", name)

	_, err = routinePrivilege("bogus")
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported privilege for routine: bogus")

	name, err = routinePrivilege("EXECUTE")
	require.NoError(t, err)
	require.Equal(t, "EXECUTE", name)

	_, err = routinePrivilege("BOGUS")
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported privilege for routine: BOGUS")
}
