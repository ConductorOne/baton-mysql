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
	entitlements, err := getEntitlementsForResource(resource, s.client)
	if err != nil {
		return nil, "", nil, err
	}

	return entitlements, "", nil, nil
}

func (s *databaseSyncer) Grants(ctx context.Context, resource *v2.Resource, pToken *pagination.Token) ([]*v2.Grant, string, annotations.Annotations, error) {
	return nil, "", nil, nil
}

func newDatabaseSyncer(c *client.Client, skipDbs map[string]struct{}) *databaseSyncer {
	return &databaseSyncer{
		resourceType: resourceTypeDatabase,
		client:       c,
		skipDbs:      skipDbs,
	}
}

func (s *databaseSyncer) Grant(ctx context.Context, principal *v2.Resource, entitlement *v2.Entitlement) (annotations.Annotations, error) {
	userResource := principal.Id.Resource
	privilege, database := extractDatabasePrivilegeAndDb(entitlement.Id)

	user := strings.Split(userResource, ":")
	userSplit := strings.Split(user[1], "@")
	userGrant := fmt.Sprintf("'%s'@'%s'", userSplit[0], userSplit[1])

	query := fmt.Sprintf("GRANT %s ON %s.* TO %s", privilege, database, userGrant)
	_, err := s.client.ExecContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("grant failed: %w", err)
	}

	return nil, nil
}

func (s *databaseSyncer) Revoke(ctx context.Context, grant *v2.Grant) (annotations.Annotations, error) {
	userResource := grant.Principal.Id.Resource
	privilege, database := extractDatabasePrivilegeAndDb(grant.Entitlement.Id)

	user := strings.Split(userResource, ":")
	userSplit := strings.Split(user[1], "@")
	userRevoke := fmt.Sprintf("'%s'@'%s'", userSplit[0], userSplit[1])

	query := fmt.Sprintf("REVOKE %s ON %s.* FROM %s", privilege, database, userRevoke)
	_, err := s.client.ExecContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("revoke failed: %w", err)
	}

	return nil, nil
}

func extractDatabasePrivilegeAndDb(entitlementID string) (string, string) {
	parts := strings.Split(entitlementID, ":")
	if len(parts) < 4 {
		return "", ""
	}

	privilegePart := parts[1]
	database := parts[3]
	priv := strings.ToUpper(strings.ReplaceAll(privilegePart, "_", " "))

	return priv, database
}
