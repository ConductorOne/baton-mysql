package connector

import (
	"context"
	"fmt"

	"github.com/conductorone/baton-mysql/pkg/client"
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/pagination"
)

type columnSyncer struct {
	resourceType *v2.ResourceType
	client       *client.Client
	expandCols   map[string]struct{}
}

func (s *columnSyncer) ResourceType(ctx context.Context) *v2.ResourceType {
	return s.resourceType
}

func (s *columnSyncer) List(
	ctx context.Context,
	parentResourceID *v2.ResourceId,
	pToken *pagination.Token,
) ([]*v2.Resource, string, annotations.Annotations, error) {
	if parentResourceID == nil || parentResourceID.ResourceType != resourceTypeTable.Id {
		return nil, "", nil, nil
	}

	columns, nextPageToken, err := s.client.ListColumns(ctx, parentResourceID, &client.Pager{Token: pToken.Token, Size: pToken.Size})
	if err != nil {
		return nil, "", nil, err
	}

	var ret []*v2.Resource
	for _, columnModel := range columns {
		if _, ok := s.expandCols[fmt.Sprintf("%s.%s", columnModel.Database, columnModel.Table)]; !ok {
			continue
		}
		ret = append(ret, &v2.Resource{
			DisplayName: fmt.Sprintf("%s.%s.%s", columnModel.Database, columnModel.Table, columnModel.Name),
			Id: &v2.ResourceId{
				ResourceType: s.resourceType.Id,
				Resource:     columnModel.ID,
			},
			ParentResourceId: parentResourceID,
		})
	}
	return ret, nextPageToken, nil, nil
}

func (s *columnSyncer) Entitlements(ctx context.Context, resource *v2.Resource, pToken *pagination.Token) ([]*v2.Entitlement, string, annotations.Annotations, error) {
	entitlements, err := getEntitlementsForResource(ctx, resource, s.client)
	if err != nil {
		return nil, "", nil, err
	}

	return entitlements, "", nil, nil
}

func (s *columnSyncer) Grants(ctx context.Context, resource *v2.Resource, pToken *pagination.Token) ([]*v2.Grant, string, annotations.Annotations, error) {
	return nil, "", nil, nil
}

func newColumnSyncer(ctx context.Context, c *client.Client, expandCols map[string]struct{}) *columnSyncer {
	return &columnSyncer{
		resourceType: resourceTypeColumn,
		client:       c,
		expandCols:   expandCols,
	}
}
