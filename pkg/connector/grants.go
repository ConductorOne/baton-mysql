package connector

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/conductorone/baton-mysql/pkg/client"
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"go.uber.org/zap"
)

func grantsForUserOrRole(
	ctx context.Context,
	c *client.Client,
	resource *v2.Resource,
	skipDbs map[string]struct{},
	expandCols map[string]struct{},
	collapseUsers bool,
) ([]*v2.Grant, error) {
	var ret []*v2.Grant
	grantMap := make(map[string]struct{})

	account, err := principalAccount(resource.Id)
	if err != nil {
		return nil, err
	}
	user, host, err := client.SplitUserHost(account)
	if err != nil {
		return nil, fmt.Errorf("baton-mysql: malformed principal ID %s: %w", resource.Id.Resource, err)
	}

	hosts := []string{host}
	// If we are collapsing users, we will want to split the host portion of the ID to inspect each real user's grants
	if collapseUsers {
		hosts = strings.Split(host, ",")
	}

	for _, host := range hosts {
		err = listGlobalGrants(ctx, resource.ParentResourceId, user, host, grantMap, c)
		if err != nil {
			return nil, err
		}

		err = listDatabaseGrants(ctx, user, host, grantMap, skipDbs, c)
		if err != nil {
			return nil, err
		}

		err = listTableGrants(ctx, user, host, grantMap, skipDbs, c)
		if err != nil {
			return nil, err
		}

		err = listColumnGrants(ctx, user, host, grantMap, skipDbs, expandCols, c)
		if err != nil {
			return nil, err
		}

		err = listProxyGrants(ctx, user, host, grantMap, c)
		if err != nil {
			return nil, err
		}

		if c.IsVersion8() {
			err = listRoleGrants(ctx, user, host, grantMap, c)
			if err != nil {
				return nil, err
			}
		}
	}

	for privResource := range grantMap {
		privParts := strings.SplitN(privResource, ":", 2)
		if len(privParts) != 2 {
			return nil, fmt.Errorf("baton-mysql: malformed priv resource id %s", privResource)
		}

		resourceParts := strings.SplitN(privParts[1], ":", 2)
		if len(resourceParts) != 2 {
			return nil, fmt.Errorf("baton-mysql: malformed resource ID %s", privParts[1])
		}

		ret = append(ret, buildGrant(privResource, resource.Id))
	}

	return ret, nil
}

// buildGrant builds the emitted v2.Grant for one entitlement-map key.
// privResource is "<entitlement-slug>:<resource-id>"; principalID is the
// synced resource's own (possibly collapsed) ID.
//
// privResource must already be a validated "<slug>:<type>:<id>" string —
// the caller (grantsForUserOrRole) checks the shape before calling;
// buildGrant does not re-validate.
func buildGrant(privResource string, principalID *v2.ResourceId) *v2.Grant {
	entitlementID := fmt.Sprintf("entitlement:%s", privResource)
	privParts := strings.SplitN(privResource, ":", 2)
	resourceParts := strings.SplitN(privParts[1], ":", 2)
	return &v2.Grant{
		Entitlement: &v2.Entitlement{
			Id: entitlementID,
			Resource: &v2.Resource{
				Id: &v2.ResourceId{
					ResourceType: resourceParts[0],
					Resource:     fmt.Sprintf("%s:%s", resourceParts[0], resourceParts[1]),
				},
			},
		},
		Principal: &v2.Resource{
			Id: principalID,
		},
		Id:          fmt.Sprintf("grant:%s:%s", entitlementID, principalID.Resource),
		Annotations: nil,
	}
}

// listGlobalgrants returns a map keyed by entitlement ID for granted global privileges.
func listGlobalGrants(
	ctx context.Context,
	resourceID *v2.ResourceId,
	user, host string,
	grantMap map[string]struct{},
	c *client.Client,
) error {
	u, err := c.GetUser(ctx, user, host)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctxzap.Extract(ctx).Debug(
				"account not found for global grants; skipping",
				zap.Error(err),
				zap.String("user", user),
				zap.String("host", host),
			)
			return nil
		}
		return fmt.Errorf("baton-mysql: unable to fetch account for global grants %s@%s: %w", user, host, err)
	}

	if c.IsVersion8() {
		globalGrants, err := c.ListGlobalGrants(ctx, u.User, u.Host)
		if err != nil {
			return err
		}

		var entitlementID string
		for _, g := range globalGrants {
			if g.WithGrant == "Y" {
				entitlementID = fmt.Sprintf("%s:%s", strings.ToLower(g.Priv)+"_with_grant", resourceID.Resource)
			} else {
				entitlementID = fmt.Sprintf("%s:%s", strings.ToLower(g.Priv), resourceID.Resource)
			}
			grantMap[entitlementID] = struct{}{}
		}
	}

	userPrivs := u.GetPrivs(ctx)
	for priv := range userPrivs {
		privKey := fmt.Sprintf("%s:%s", priv, resourceID.Resource)
		grantMap[privKey] = struct{}{}
	}

	return nil
}

// listDatabaseGrants returns a map keyed by entitlement ID for granted global privileges.
func listDatabaseGrants(
	ctx context.Context,
	user, host string,
	grantMap map[string]struct{},
	skipDbs map[string]struct{},
	c *client.Client,
) error {
	dbGrants, err := c.ListDatabaseGrants(ctx, user, host)
	if err != nil {
		return err
	}

	var entitlementID string
	for _, g := range dbGrants {
		if _, ok := skipDbs[g.Database]; ok {
			continue
		}
		for priv := range g.GetPrivs(ctx) {
			entitlementID = fmt.Sprintf("%s:%s", strings.ToLower(priv), g.Id)
			grantMap[entitlementID] = struct{}{}
		}
	}

	return nil
}

// listDatabaseGrants returns a map keyed by entitlement ID for granted table privileges.
func listTableGrants(
	ctx context.Context,
	user, host string,
	grantMap map[string]struct{},
	skipDbs map[string]struct{},
	c *client.Client,
) error {
	tableGrants, err := c.ListTableGrants(ctx, user, host)
	if err != nil {
		return err
	}

	var entitlementID string
	for _, g := range tableGrants {
		if _, ok := skipDbs[g.Database]; ok {
			continue
		}
		for priv := range g.GetPrivs(ctx) {
			entitlementID = fmt.Sprintf("%s:%s", strings.ToLower(priv), g.Id)
			grantMap[entitlementID] = struct{}{}
		}
	}

	return nil
}

// listColumnGrants returns a map keyed by entitlement ID for granted column privileges.
func listColumnGrants(
	ctx context.Context,
	user, host string,
	grantMap map[string]struct{},
	skipDbs map[string]struct{},
	expandCols map[string]struct{},
	c *client.Client,
) error {
	columnGrants, err := c.ListColumnGrants(ctx, user, host)
	if err != nil {
		return err
	}

	var entitlementID string
	for _, g := range columnGrants {
		if _, ok := skipDbs[g.Database]; ok {
			continue
		}

		// grantID defaults to the table for columns. If the column's table is expanded, set the grantID to the column
		grantID := g.TableID()
		if _, ok := expandCols[fmt.Sprintf("%s.%s", g.Database, g.Table)]; ok {
			grantID = g.Id
		}

		for priv := range g.GetPrivs(ctx) {
			entitlementID = fmt.Sprintf("%s:%s", strings.ToLower(priv), grantID)
			grantMap[entitlementID] = struct{}{}
		}
	}

	return nil
}

// listProxyGrants returns a map keyed by entitlement ID for granted proxy privileges.
func listProxyGrants(ctx context.Context, user, host string, grantMap map[string]struct{}, c *client.Client) error {
	proxyGrants, err := c.ListProxyGrants(ctx, user, host)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctxzap.Extract(ctx).Debug(
				"account not found for proxy grants; skipping",
				zap.Error(err),
				zap.String("user", user),
				zap.String("host", host),
			)
			return nil
		}
		return fmt.Errorf("baton-mysql: unable to fetch proxy grants for %s@%s: %w", user, host, err)
	}

	for _, g := range proxyGrants {
		grantMap[fmt.Sprintf("proxy:%s", g.Id)] = struct{}{}
		if g.WithGrant == 1 {
			grantMap[fmt.Sprintf("proxy_with_grant:%s", g.Id)] = struct{}{}
		}
	}

	return nil
}

// listRoleGrants returns a map keyed by entitlement ID for granted role edges.
func listRoleGrants(ctx context.Context, user, host string, grantMap map[string]struct{}, c *client.Client) error {
	roleGrants, err := c.ListRoleGrants(ctx, user, host)
	if err != nil {
		return err
	}

	for _, g := range roleGrants {
		for _, key := range roleGrantKeys(g.Id, g.WithGrant == "Y") {
			grantMap[key] = struct{}{}
		}
	}

	return nil
}

// roleGrantKeys returns the entitlement-map keys for one role edge:
// the membership key always, and the with-grant key additionally
// (never as a replacement) when the edge carries WITH ADMIN OPTION.
func roleGrantKeys(id string, withAdmin bool) []string {
	keys := []string{fmt.Sprintf("role_assignment:%s", id)}
	if withAdmin {
		keys = append(keys, fmt.Sprintf("role_assignment_with_grant:%s", id))
	}
	return keys
}
