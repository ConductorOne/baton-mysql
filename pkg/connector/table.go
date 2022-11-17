package connector

import (
	"context"
	"fmt"

	"github.com/conductorone/baton-mysql/pkg/client"
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/pagination"
)

type tableSyncer struct {
	resourceType *v2.ResourceType
	client       *client.Client
	expandCols   map[string]struct{}
}

func (s *tableSyncer) ResourceType(ctx context.Context) *v2.ResourceType {
	return s.resourceType
}

func (s *tableSyncer) List(
	ctx context.Context,
	parentResourceID *v2.ResourceId,
	pToken *pagination.Token,
) ([]*v2.Resource, string, annotations.Annotations, error) {
	if parentResourceID == nil || parentResourceID.ResourceType != resourceTypeDatabase.Id {
		return nil, "", nil, nil
	}

	tables, nextPageToken, err := s.client.ListTables(ctx, parentResourceID, &client.Pager{Token: pToken.Token, Size: pToken.Size})
	if err != nil {
		return nil, "", nil, err
	}

	var annos annotations.Annotations
	if len(s.expandCols) > 0 {
		annos.Append(&v2.ChildResourceType{ResourceTypeId: resourceTypeColumn.Id})
	}

	var ret []*v2.Resource
	for _, tableModel := range tables {
		ret = append(ret, &v2.Resource{
			DisplayName: fmt.Sprintf("%s.%s", tableModel.Database, tableModel.Name),
			Id: &v2.ResourceId{
				ResourceType: s.resourceType.Id,
				Resource:     tableModel.ID,
			},
			Annotations:      annos,
			ParentResourceId: parentResourceID,
		})
	}
	return ret, nextPageToken, nil, nil
}

func (s *tableSyncer) Entitlements(ctx context.Context, resource *v2.Resource, pToken *pagination.Token) ([]*v2.Entitlement, string, annotations.Annotations, error) {
	entitlements, err := getEntitlementsForResource(ctx, resource, s.client)
	if err != nil {
		return nil, "", nil, err
	}

	return entitlements, "", nil, nil
}

func (s *tableSyncer) Grants(ctx context.Context, resource *v2.Resource, pToken *pagination.Token) ([]*v2.Grant, string, annotations.Annotations, error) {
	return nil, "", nil, nil
}

func newTableSyncer(ctx context.Context, c *client.Client, expandCols map[string]struct{}) *tableSyncer {
	return &tableSyncer{
		resourceType: resourceTypeTable,
		client:       c,
		expandCols:   expandCols,
	}
}
