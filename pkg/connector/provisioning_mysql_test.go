package connector

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	_ "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/require"

	"github.com/conductorone/baton-mysql/pkg/client"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/pagination"
)

func liveMySQLCtx(t *testing.T) (context.Context, *client.Client, *sql.DB) {
	t.Helper()
	dsn := os.Getenv("BATON_CONNECTION_STRING")
	if dsn == "" {
		t.Skip("BATON_CONNECTION_STRING not set; skipping live MySQL test")
	}
	ctx := context.Background()
	c, err := client.New(ctx, dsn)
	require.NoError(t, err)
	db, err := sql.Open("mysql", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return ctx, c, db
}

func liveTestUser(t *testing.T, c *client.Client) *v2.Resource {
	t.Helper()
	ctx := context.Background()
	_, err := c.ExecContext(ctx, "DROP USER IF EXISTS 'baton_prov_test'@'%'")
	require.NoError(t, err)
	_, err = c.ExecContext(ctx, "CREATE USER 'baton_prov_test'@'%' IDENTIFIED BY 'provtest123'")
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = c.ExecContext(ctx, "DROP USER IF EXISTS 'baton_prov_test'@'%'")
	})
	return &v2.Resource{Id: &v2.ResourceId{ResourceType: resourceTypeUser.Id, Resource: "user:baton_prov_test@%"}}
}

func showGrants(t *testing.T, db *sql.DB) []string {
	t.Helper()
	rows, err := db.Query("SHOW GRANTS FOR 'baton_prov_test'@'%'")
	require.NoError(t, err)
	defer rows.Close()

	var grants []string
	for rows.Next() {
		var g string
		require.NoError(t, rows.Scan(&g))
		grants = append(grants, g)
	}
	require.NoError(t, rows.Err())
	return grants
}

func grantsContain(grants []string, needle string) bool {
	for _, g := range grants {
		if strings.Contains(g, needle) {
			return true
		}
	}
	return false
}

func TestProvisioningLive(t *testing.T) {
	ctx, c, db := liveMySQLCtx(t)
	principal := liveTestUser(t, c)

	s := newServerSyncer(c)

	// Static privilege at server scope round-trips.
	_, err := s.Grant(ctx, principal, entitlement("entitlement:show_databases:server:srv1"))
	require.NoError(t, err)
	grants := showGrants(t, db)
	require.True(t, grantsContain(grants, "SHOW DATABASES"), "grants: %v", grants)

	_, err = s.Revoke(ctx, grant("entitlement:show_databases:server:srv1", principal))
	require.NoError(t, err)
	grants = showGrants(t, db)
	require.False(t, grantsContain(grants, "SHOW DATABASES"), "grants: %v", grants)

	// Dynamic privilege with grant option round-trips.
	_, err = s.Grant(ctx, principal, entitlement("entitlement:backup_admin_with_grant:server:srv1"))
	require.NoError(t, err)
	grants = showGrants(t, db)
	require.True(t, grantsContain(grants, "BACKUP_ADMIN"), "grants: %v", grants)
	require.True(t, grantsContain(grants, "WITH GRANT OPTION"), "grants: %v", grants)

	_, err = s.Revoke(ctx, grant("entitlement:backup_admin_with_grant:server:srv1", principal))
	require.NoError(t, err)
	grants = showGrants(t, db)
	require.False(t, grantsContain(grants, "BACKUP_ADMIN"), "grants: %v", grants)

	// Table scope round-trips with the exact SQL name.
	_, err = c.ExecContext(ctx, "CREATE DATABASE IF NOT EXISTS testdb")
	require.NoError(t, err)
	_, err = c.ExecContext(ctx, "CREATE TABLE IF NOT EXISTS testdb.prov_test_tbl (id INT)")
	require.NoError(t, err)
	ts := newTableSyncer(c, map[string]struct{}{})
	_, err = ts.Grant(ctx, principal, entitlement("entitlement:create_view:table:testdb.prov_test_tbl"))
	require.NoError(t, err)
	grants = showGrants(t, db)
	require.True(t, grantsContain(grants, "CREATE VIEW"), "grants: %v", grants)
	require.True(t, grantsContain(grants, "prov_test_tbl"), "grants: %v", grants)

	_, err = ts.Revoke(ctx, grant("entitlement:create_view:table:testdb.prov_test_tbl", principal))
	require.NoError(t, err)
	grants = showGrants(t, db)
	require.False(t, grantsContain(grants, "CREATE VIEW"), "grants: %v", grants)

	// Grant option at database scope emits GRANT OPTION, never GRANT GRANT.
	ds := newDatabaseSyncer(c, map[string]struct{}{})
	_, err = ds.Grant(ctx, principal, entitlement("entitlement:grant:database:db1"))
	require.NoError(t, err)
	grants = showGrants(t, db)
	require.True(t, grantsContain(grants, "GRANT OPTION"), "grants: %v", grants)
	for _, g := range grants {
		require.NotContains(t, g, "GRANT GRANT")
	}
	_, err = ds.Revoke(ctx, grant("entitlement:grant:database:db1", principal))
	require.NoError(t, err)
}

func TestSyncInvariance(t *testing.T) {
	ctx, c, _ := liveMySQLCtx(t)
	require.True(t, c.IsVersion8())

	serverResource := &v2.Resource{Id: &v2.ResourceId{ResourceType: resourceTypeServer.Id, Resource: "server:localhost"}}
	checkInvariance(t, ctx, c, newServerSyncer(c), serverResource)
	tableResource := &v2.Resource{Id: &v2.ResourceId{ResourceType: resourceTypeTable.Id, Resource: "table:testdb.prov_test_tbl"}}
	checkInvariance(t, ctx, c, newTableSyncer(c, map[string]struct{}{}), tableResource)
}

type entitlementsSource interface {
	Entitlements(ctx context.Context, resource *v2.Resource, pToken *pagination.Token) ([]*v2.Entitlement, string, annotations.Annotations, error)
}

func checkInvariance(t *testing.T, ctx context.Context, c *client.Client, s entitlementsSource, resource *v2.Resource) {
	t.Helper()
	tmpls, ok := entitlementsByResourceType[resource.Id.ResourceType]
	require.True(t, ok)
	require.NotEmpty(t, tmpls)

	ents, _, _, err := s.Entitlements(ctx, resource, nil)
	require.NoError(t, err)
	require.Len(t, ents, len(tmpls))

	expected := make([]*v2.Entitlement, 0, len(tmpls))
	for _, tmpl := range tmpls {
		expected = append(expected, &v2.Entitlement{
			Id:          fmt.Sprintf("entitlement:%s:%s", tmpl.ID, resource.Id.Resource),
			Slug:        strings.ToLower(getEntitlementSlug(tmpl, resource)),
			DisplayName: getEntitlementDisplayName(tmpl, resource),
			Description: getEntitlementDescription(tmpl, resource),
			Purpose:     tmpl.entitlement.Purpose,
		})
	}
	sort.Slice(expected, func(i, j int) bool { return expected[i].Id < expected[j].Id })
	sorted := make([]*v2.Entitlement, len(ents))
	copy(sorted, ents)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Id < sorted[j].Id })

	for i := range expected {
		require.Equal(t, expected[i].Id, sorted[i].Id)
		require.Equal(t, expected[i].Slug, sorted[i].Slug)
		require.Equal(t, expected[i].DisplayName, sorted[i].DisplayName)
		require.Equal(t, expected[i].Description, sorted[i].Description)
		require.Equal(t, expected[i].Purpose, sorted[i].Purpose)
	}
}
