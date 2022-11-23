package connector

import (
	"context"
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

	parts := strings.Split(strings.TrimPrefix(resource.Id.Resource, fmt.Sprintf("%s:", resource.Id.ResourceType)), "@")
	if len(parts) != 2 {
		return nil, fmt.Errorf("malformed principal ID")
	}
	user := parts[0]

	hosts := []string{parts[1]}
	// If we are collapsing users, we will want to split the host portion of the ID to inspect each real user's grants
	if collapseUsers {
		hosts = strings.Split(parts[1], ",")
	}

	var err error
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
			return nil, fmt.Errorf("malformed priv resource id")
		}

		resourceParts := strings.SplitN(privParts[1], ":", 2)
		if len(resourceParts) != 2 {
			return nil, fmt.Errorf("malformed resource ID")
		}

		entitlementID := fmt.Sprintf("entitlement:%s", privResource)
		ret = append(ret, &v2.Grant{
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
				Id: resource.Id,
			},
			Id:          fmt.Sprintf("grant:%s:%s", entitlementID, resource.Id.Resource),
			Annotations: nil,
		})
	}

	return ret, nil
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
		ctxzap.Extract(ctx).Error(
			"unable to fetch to user for global grant. Ignoring grants",
			zap.Error(err),
			zap.String("user", user),
			zap.String("host", host),
		)
		return nil
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
		ctxzap.Extract(ctx).Error(
			"unable to fetch to proxy grants. ignoring",
			zap.Error(err),
			zap.String("user", user),
			zap.String("host", host),
		)
		return nil
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
		grantMap[fmt.Sprintf("role_assignment:%s", g.Id)] = struct{}{}
		if g.WithGrant == "Y" {
			grantMap[fmt.Sprintf("role_assignment_with_grant:%s", g.Id)] = struct{}{}
		}
	}

	return nil
}
