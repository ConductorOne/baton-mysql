package connector

import (
	"context"
	"fmt"
	"strings"

	"github.com/conductorone/baton-mysql/pkg/client"
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/pagination"
)

type roleSyncer struct {
	resourceType *v2.ResourceType
	client       *client.Client
	skipDbs      map[string]struct{}
	expandCols   map[string]struct{}
}

func (s *roleSyncer) ResourceType(ctx context.Context) *v2.ResourceType {
	return s.resourceType
}

func (s *roleSyncer) List(
	ctx context.Context,
	parentResourceID *v2.ResourceId,
	pToken *pagination.Token,
) ([]*v2.Resource, string, annotations.Annotations, error) {
	if parentResourceID == nil || parentResourceID.ResourceType != resourceTypeServer.Id {
		return nil, "", nil, nil
	}

	users, nextPageToken, err := s.client.ListUsers(ctx, s.resourceType.Id, &client.Pager{Token: pToken.Token, Size: pToken.Size}, false)
	if err != nil {
		return nil, "", nil, err
	}

	var ret []*v2.Resource
	for _, u := range users {
		var annos annotations.Annotations

		ret = append(ret, &v2.Resource{
			DisplayName: fmt.Sprintf("%s@%s", u.User, u.Host),
			Id: &v2.ResourceId{
				ResourceType: s.resourceType.Id,
				Resource:     u.GetID(),
			},
			Annotations:      annos,
			ParentResourceId: parentResourceID,
		})
	}

	return ret, nextPageToken, nil, nil
}

func (s *roleSyncer) Entitlements(ctx context.Context, resource *v2.Resource, pToken *pagination.Token) ([]*v2.Entitlement, string, annotations.Annotations, error) {
	entitlements, err := getEntitlementsForResource(resource, s.client)
	if err != nil {
		return nil, "", nil, err
	}

	return entitlements, "", nil, nil
}

func (s *roleSyncer) Grants(ctx context.Context, resource *v2.Resource, pToken *pagination.Token) ([]*v2.Grant, string, annotations.Annotations, error) {
	grants, err := grantsForUserOrRole(ctx, s.client, resource, s.skipDbs, s.expandCols, false)
	if err != nil {
		return nil, "", nil, err
	}

	return grants, "", nil, nil
}

func newRoleSyncer(c *client.Client, skipDbs map[string]struct{}, expandCols map[string]struct{}) *roleSyncer {
	return &roleSyncer{
		resourceType: resourceTypeRole,
		client:       c,
		skipDbs:      skipDbs,
		expandCols:   expandCols,
	}
}

func (s *roleSyncer) Grant(ctx context.Context, principal *v2.Resource, entitlement *v2.Entitlement) (annotations.Annotations, error) {
	if principal.Id.ResourceType != resourceTypeUser.Id {
		return nil, fmt.Errorf("can only grant role entitlements to users")
	}

	parts := strings.Split(entitlement.Id, ":")
	if len(parts) != 4 {
		return nil, fmt.Errorf("invalid entitlement ID: %s", entitlement.Id)
	}
	privilege := parts[1]
	roleName := parts[3]

	userSplit := strings.Split(principal.Id.Resource, ":")
	if len(userSplit) != 2 {
		return nil, fmt.Errorf("invalid principal ID: %s", principal.Id.Resource)
	}
	user := userSplit[1]

	err := s.client.GrantRolePrivilege(ctx, roleName, user, privilege)
	if err != nil {
		return nil, fmt.Errorf("failed to grant %s on role %s to user %s: %w", privilege, roleName, user, err)
	}

	return nil, nil
}

func (s *roleSyncer) Revoke(ctx context.Context, grant *v2.Grant) (annotations.Annotations, error) {
	parts := strings.Split(grant.Entitlement.Id, ":")
	if len(parts) != 4 {
		return nil, fmt.Errorf("invalid entitlement ID: %s", grant.Entitlement.Id)
	}
	privilege := parts[1]
	roleName := parts[3]

	userSplit := strings.Split(grant.Principal.Id.Resource, ":")
	if len(userSplit) != 2 {
		return nil, fmt.Errorf("invalid principal ID: %s", grant.Principal.Id.Resource)
	}
	user := userSplit[1]

	err := s.client.RevokeRolePrivilege(ctx, roleName, user, privilege)
	if err != nil {
		return nil, fmt.Errorf("failed to revoke %s on role %s from user %s: %w", privilege, roleName, user, err)
	}

	return nil, nil
}
