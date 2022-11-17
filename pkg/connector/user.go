package connector

import (
	"context"
	"fmt"

	"github.com/conductorone/baton-mysql/pkg/client"
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/pagination"
	"github.com/conductorone/baton-sdk/pkg/sdk"
)

type userSyncer struct {
	resourceType  *v2.ResourceType
	client        *client.Client
	skipDbs       map[string]struct{}
	expandCols    map[string]struct{}
	collapseUsers bool
}

func (s *userSyncer) ResourceType(ctx context.Context) *v2.ResourceType {
	return s.resourceType
}

func (s *userSyncer) List(
	ctx context.Context,
	parentResourceID *v2.ResourceId,
	pToken *pagination.Token,
) ([]*v2.Resource, string, annotations.Annotations, error) {
	if parentResourceID == nil || parentResourceID.ResourceType != resourceTypeServer.Id {
		return nil, "", nil, nil
	}

	users, nextPageToken, err := s.client.ListUsers(ctx, s.resourceType.Id, &client.Pager{Token: pToken.Token, Size: pToken.Size}, s.collapseUsers)
	if err != nil {
		return nil, "", nil, err
	}

	var ret []*v2.Resource
	for _, u := range users {
		var annos annotations.Annotations

		ut, err := sdk.NewUserTrait("", v2.UserTrait_Status_STATUS_ENABLED, nil, map[string]interface{}{
			"user":       u.User,
			"host":       u.Host,
			"first_name": fmt.Sprintf("%s@%s", u.User, u.Host),
			"user_id":    fmt.Sprintf("%s@%s", u.User, u.Host),
		})
		if err != nil {
			return nil, "", nil, err
		}
		annos.Append(ut)

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

func (s *userSyncer) Entitlements(ctx context.Context, resource *v2.Resource, pToken *pagination.Token) ([]*v2.Entitlement, string, annotations.Annotations, error) {
	entitlements, err := getEntitlementsForResource(ctx, resource, s.client)
	if err != nil {
		return nil, "", nil, err
	}

	return entitlements, "", nil, nil
}

func (s *userSyncer) Grants(ctx context.Context, resource *v2.Resource, pToken *pagination.Token) ([]*v2.Grant, string, annotations.Annotations, error) {
	grants, err := grantsForUserOrRole(ctx, s.client, resource, s.skipDbs, s.expandCols, s.collapseUsers)
	if err != nil {
		return nil, "", nil, err
	}

	return grants, "", nil, nil
}

func newUserSyncer(ctx context.Context, c *client.Client, skipDbs map[string]struct{}, expandCols map[string]struct{}, collapseUsers bool) *userSyncer {
	return &userSyncer{
		resourceType:  resourceTypeUser,
		client:        c,
		skipDbs:       skipDbs,
		expandCols:    expandCols,
		collapseUsers: collapseUsers,
	}
}
