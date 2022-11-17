package connector

import (
	"context"

	"github.com/conductorone/baton-mysql/pkg/client"
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/pagination"
)

type databaseSyncer struct {
	resourceType *v2.ResourceType
	client       *client.Client
	skipDbs      map[string]struct{}
}

func (s *databaseSyncer) ResourceType(ctx context.Context) *v2.ResourceType {
	return s.resourceType
}

func (s *databaseSyncer) List(
	ctx context.Context,
	parentResourceID *v2.ResourceId,
	pToken *pagination.Token,
) ([]*v2.Resource, string, annotations.Annotations, error) {
	if parentResourceID == nil || parentResourceID.ResourceType != resourceTypeServer.Id {
		return nil, "", nil, nil
	}

	databases, nextPageToken, err := s.client.ListDatabases(ctx, &client.Pager{Token: pToken.Token, Size: pToken.Size})
	if err != nil {
		return nil, "", nil, err
	}

	var annos annotations.Annotations
	annos.Append(&v2.ChildResourceType{ResourceTypeId: resourceTypeTable.Id})
	annos.Append(&v2.ChildResourceType{ResourceTypeId: resourceTypeRoutine.Id})

	var ret []*v2.Resource
	for _, dbModel := range databases {
		if _, ok := s.skipDbs[dbModel.Name]; ok {
			continue
		}
		ret = append(ret, &v2.Resource{
			DisplayName: dbModel.Name,
			Id: &v2.ResourceId{
				ResourceType: s.resourceType.Id,
				Resource:     dbModel.ID,
			},
			ParentResourceId: parentResourceID,
			Annotations:      annos,
		})
	}
	return ret, nextPageToken, nil, nil
}

func (s *databaseSyncer) Entitlements(ctx context.Context, resource *v2.Resource, pToken *pagination.Token) ([]*v2.Entitlement, string, annotations.Annotations, error) {
	entitlements, err := getEntitlementsForResource(ctx, resource, s.client)
	if err != nil {
		return nil, "", nil, err
	}

	return entitlements, "", nil, nil
}

func (s *databaseSyncer) Grants(ctx context.Context, resource *v2.Resource, pToken *pagination.Token) ([]*v2.Grant, string, annotations.Annotations, error) {
	return nil, "", nil, nil
}

func newDatabaseSyncer(ctx context.Context, c *client.Client, skipDbs map[string]struct{}) *databaseSyncer {
	return &databaseSyncer{
		resourceType: resourceTypeDatabase,
		client:       c,
		skipDbs:      skipDbs,
	}
}
