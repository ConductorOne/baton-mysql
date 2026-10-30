package connector

import (
	"context"
	"fmt"

	"github.com/conductorone/baton-mysql/pkg/client"
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/pagination"
)

type serverSyncer struct {
	resourceType *v2.ResourceType
	client       *client.Client
}

func (s *serverSyncer) ResourceType(ctx context.Context) *v2.ResourceType {
	return s.resourceType
}

func (s *serverSyncer) List(
	ctx context.Context,
	parentResourceID *v2.ResourceId,
	pToken *pagination.Token,
) ([]*v2.Resource, string, annotations.Annotations, error) {
	server, err := s.client.GetServerInfo(ctx)
	if err != nil {
		return nil, "", nil, err
	}

	var annos annotations.Annotations
	annos.Append(&v2.ChildResourceType{ResourceTypeId: resourceTypeDatabase.Id})
	annos.Append(&v2.ChildResourceType{ResourceTypeId: resourceTypeUser.Id})

	if s.client.IsVersion8() {
		annos.Append(&v2.ChildResourceType{ResourceTypeId: resourceTypeRole.Id})
	}

	return []*v2.Resource{
		{
			Id: &v2.ResourceId{
				ResourceType: resourceTypeServer.Id,
				Resource:     server.ID,
			},
			DisplayName: server.Name,
			Annotations: annos,
		},
	}, "", nil, nil
}

func (s *serverSyncer) Entitlements(ctx context.Context, resource *v2.Resource, pToken *pagination.Token) ([]*v2.Entitlement, string, annotations.Annotations, error) {
	entitlements, err := getEntitlementsForResource(resource, s.client)
	if err != nil {
		return nil, "", nil, err
	}

	return entitlements, "", nil, nil
}

func (s *serverSyncer) Grants(ctx context.Context, resource *v2.Resource, pToken *pagination.Token) ([]*v2.Grant, string, annotations.Annotations, error) {
	return nil, "", nil, nil
}

func newServerSyncer(c *client.Client) *serverSyncer {
	return &serverSyncer{
		resourceType: resourceTypeServer,
		client:       c,
	}
}

func (s *serverSyncer) Grant(ctx context.Context, principal *v2.Resource, entitlement *v2.Entitlement) (annotations.Annotations, error) {
	privilege, withGrant, err := resolveServerPrivilege(entitlement.Id, false)
	if err != nil {
		return nil, err
	}

	userStr, err := principalAccount(principal.Id)
	if err != nil {
		return nil, err
	}

	err = s.client.GrantServerPrivilege(ctx, userStr, privilege, withGrant)
	if err != nil {
		return nil, fmt.Errorf("grant failed: %w", err)
	}

	return nil, nil
}

func (s *serverSyncer) Revoke(ctx context.Context, grant *v2.Grant) (annotations.Annotations, error) {
	sqlName, _, err := resolveServerPrivilege(grant.Entitlement.Id, true)
	if err != nil {
		return nil, err
	}

	userStr, err := principalAccount(grant.Principal.Id)
	if err != nil {
		return nil, err
	}

	// Revoke uses the base privilege: revoking drops the grant option along with the privilege.
	err = s.client.RevokeServerPrivilege(ctx, userStr, sqlName)
	if err != nil {
		return nil, fmt.Errorf("revoke failed: %w", err)
	}

	return nil, nil
}
