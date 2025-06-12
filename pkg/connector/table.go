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
	entitlements, err := getEntitlementsForResource(resource, s.client)
	if err != nil {
		return nil, "", nil, err
	}

	return entitlements, "", nil, nil
}

func (s *tableSyncer) Grants(ctx context.Context, resource *v2.Resource, pToken *pagination.Token) ([]*v2.Grant, string, annotations.Annotations, error) {
	return nil, "", nil, nil
}

func newTableSyncer(c *client.Client, expandCols map[string]struct{}) *tableSyncer {
	return &tableSyncer{
		resourceType: resourceTypeTable,
		client:       c,
		expandCols:   expandCols,
	}
}

func (s *tableSyncer) Grant(ctx context.Context, principal *v2.Resource, entitlement *v2.Entitlement) (annotations.Annotations, error) {
	if principal.Id.ResourceType != resourceTypeUser.Id {
		return nil, fmt.Errorf("can only grant table permissions to users")
	}

	parts := strings.Split(entitlement.Id, ":")
	if len(parts) != 4 {
		return nil, fmt.Errorf("invalid entitlement ID: %s", entitlement.Id)
	}
	privilege := parts[1]
	tableID := parts[3]

	userName := strings.Split(principal.Id.Resource, ":")
	if len(userName) != 2 {
		return nil, fmt.Errorf("invalid principal ID: %s", principal.Id.Resource)
	}

	err := s.client.GrantTablePrivilege(ctx, tableID, userName[1], privilege)
	if err != nil {
		return nil, fmt.Errorf("failed to grant %s on %s to %s: %w", privilege, tableID, principal.Id.Resource, err)
	}

	return nil, nil
}

func (s *tableSyncer) Revoke(ctx context.Context, grant *v2.Grant) (annotations.Annotations, error) {
	parts := strings.Split(grant.Entitlement.Id, ":")
	if len(parts) != 4 {
		return nil, fmt.Errorf("invalid entitlement ID: %s", grant.Entitlement.Id)
	}
	privilege := parts[1]
	tableID := parts[3]

	userName := strings.Split(grant.Principal.Id.Resource, ":")
	if len(userName) != 2 {
		return nil, fmt.Errorf("invalid principal ID: %s", grant.Principal.Id.Resource)
	}

	err := s.client.RevokeTablePrivilege(ctx, tableID, userName[1], privilege)
	if err != nil {
		return nil, fmt.Errorf("failed to revoke %s on %s from %s: %w", privilege, tableID, grant.Principal.Id.Resource, err)
	}

	return nil, nil
}
