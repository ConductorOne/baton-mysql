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
	entitlements, err := getEntitlementsForResource(resource, s.client)
	if err != nil {
		return nil, "", nil, err
	}

	return entitlements, "", nil, nil
}

func (s *columnSyncer) Grants(ctx context.Context, resource *v2.Resource, pToken *pagination.Token) ([]*v2.Grant, string, annotations.Annotations, error) {
	return nil, "", nil, nil
}

func (s *columnSyncer) Grant(ctx context.Context, principal *v2.Resource, entitlement *v2.Entitlement) (annotations.Annotations, error) {
	if principal.Id.ResourceType != resourceTypeUser.Id {
		return nil, fmt.Errorf("can only grant column permissions to users")
	}

	parts := strings.Split(entitlement.Id, ":")
	privilege := parts[1]

	columnParts := strings.Split(parts[3], ".")
	tableName := fmt.Sprintf("%s.%s", columnParts[0], columnParts[1])
	columnName := columnParts[2]

	user := strings.Split(principal.Id.Resource, ":")[1]

	err := s.client.GrantColumnPrivilege(ctx, tableName, columnName, user, privilege)
	if err != nil {
		return nil, fmt.Errorf("failed to grant %s on %s to %s: %w", privilege, entitlement.Id, principal.Id.Resource, err)
	}

	return nil, nil
}

func (s *columnSyncer) Revoke(ctx context.Context, grant *v2.Grant) (annotations.Annotations, error) {
	parts := strings.Split(grant.Entitlement.Id, ":")
	if len(parts) != 4 {
		return nil, fmt.Errorf("invalid entitlement ID: %s", grant.Entitlement.Id)
	}
	privilege := parts[1]
	columnID := parts[3]

	idParts := strings.Split(columnID, ".")
	if len(idParts) != 3 {
		return nil, fmt.Errorf("invalid column ID format: %s", columnID)
	}
	table := fmt.Sprintf("%s.%s", idParts[0], idParts[1])
	column := idParts[2]

	userParts := strings.Split(grant.Principal.Id.Resource, ":")
	if len(userParts) != 2 {
		return nil, fmt.Errorf("invalid principal ID: %s", grant.Principal.Id.Resource)
	}
	user := userParts[1]

	err := s.client.RevokeColumnPrivilege(ctx, table, column, user, privilege)
	if err != nil {
		return nil, fmt.Errorf("failed to revoke %s on %s.%s from %s: %w", privilege, table, column, user, err)
	}

	return nil, nil
}

func newColumnSyncer(c *client.Client, expandCols map[string]struct{}) *columnSyncer {
	return &columnSyncer{
		resourceType: resourceTypeColumn,
		client:       c,
		expandCols:   expandCols,
	}
}
