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

type routineSyncer struct {
	resourceType *v2.ResourceType
	client       *client.Client
}

func (s *routineSyncer) ResourceType(ctx context.Context) *v2.ResourceType {
	return s.resourceType
}

func (s *routineSyncer) List(
	ctx context.Context,
	parentResourceID *v2.ResourceId,
	pToken *pagination.Token,
) ([]*v2.Resource, string, annotations.Annotations, error) {
	if parentResourceID == nil || parentResourceID.ResourceType != resourceTypeDatabase.Id {
		return nil, "", nil, nil
	}

	routines, nextPageToken, err := s.client.ListRoutines(ctx, parentResourceID, &client.Pager{Token: pToken.Token, Size: pToken.Size})
	if err != nil {
		return nil, "", nil, err
	}

	var ret []*v2.Resource
	for _, routineModel := range routines {
		ret = append(ret, &v2.Resource{
			DisplayName: fmt.Sprintf("%s.%s", routineModel.Database, routineModel.Name),
			Id: &v2.ResourceId{
				ResourceType: s.resourceType.Id,
				Resource:     routineModel.ID,
			},
			ParentResourceId: parentResourceID,
		})
	}

	return ret, nextPageToken, nil, nil
}

func (s *routineSyncer) Entitlements(ctx context.Context, resource *v2.Resource, pToken *pagination.Token) ([]*v2.Entitlement, string, annotations.Annotations, error) {
	entitlements, err := getEntitlementsForResource(resource, s.client)
	if err != nil {
		return nil, "", nil, err
	}

	return entitlements, "", nil, nil
}

func (s *routineSyncer) Grants(ctx context.Context, resource *v2.Resource, pToken *pagination.Token) ([]*v2.Grant, string, annotations.Annotations, error) {
	return nil, "", nil, nil
}

func newRoutineSyncer(c *client.Client) *routineSyncer {
	return &routineSyncer{
		resourceType: resourceTypeRoutine,
		client:       c,
	}
}
func (s *routineSyncer) Grant(ctx context.Context, principal *v2.Resource, entitlement *v2.Entitlement) (annotations.Annotations, error) {
	if principal.Id.ResourceType != resourceTypeUser.Id {
		return nil, fmt.Errorf("can only grant routine permissions to users")
	}

	parts := strings.Split(entitlement.Id, ":")
	if len(parts) != 4 {
		return nil, fmt.Errorf("invalid entitlement ID: %s", entitlement.Id)
	}

	rawPrivilege := parts[1]
	resourceKind := parts[2]
	fullRoutineName := parts[3]

	if resourceKind != "routine" {
		return nil, fmt.Errorf("unsupported resource kind in entitlement ID: %s", entitlement.Id)
	}

	var privilege string
	switch strings.ToLower(rawPrivilege) {
	case "execute":
		privilege = "EXECUTE"
	case "alter_routine":
		privilege = "ALTER ROUTINE"
	default:
		return nil, fmt.Errorf("unsupported privilege for routine: %s", rawPrivilege)
	}

	schemaRoutine := strings.Split(fullRoutineName, ".")
	if len(schemaRoutine) != 2 {
		return nil, fmt.Errorf("invalid routine name, expected schema.name: %s", fullRoutineName)
	}
	schema := schemaRoutine[0]
	routineName := schemaRoutine[1]

	userSplit := strings.Split(principal.Id.Resource, ":")
	if len(userSplit) != 2 {
		return nil, fmt.Errorf("invalid principal ID: %s", principal.Id.Resource)
	}
	user := userSplit[1]

	err := s.client.GrantRoutinePrivilege(ctx, privilege, schema, routineName, user)
	if err != nil {
		return nil, fmt.Errorf("failed to grant %s on %s.%s to %s: %w", privilege, schema, routineName, user, err)
	}

	return nil, nil
}
func (s *routineSyncer) Revoke(ctx context.Context, grant *v2.Grant) (annotations.Annotations, error) {
	parts := strings.Split(grant.Entitlement.Id, ":")
	if len(parts) != 4 {
		return nil, fmt.Errorf("invalid entitlement ID: %s", grant.Entitlement.Id)
	}

	rawPrivilege := parts[1]
	resourceKind := parts[2]
	fullRoutineName := parts[3]

	if resourceKind != "routine" {
		return nil, fmt.Errorf("unsupported resource kind in entitlement ID: %s", grant.Entitlement.Id)
	}

	var privilege string
	switch strings.ToLower(rawPrivilege) {
	case "execute":
		privilege = "EXECUTE"
	case "alter_routine":
		privilege = "ALTER ROUTINE"
	default:
		return nil, fmt.Errorf("unsupported privilege for routine: %s", rawPrivilege)
	}

	schemaRoutine := strings.Split(fullRoutineName, ".")
	if len(schemaRoutine) != 2 {
		return nil, fmt.Errorf("invalid routine name, expected schema.name: %s", fullRoutineName)
	}
	schema := schemaRoutine[0]
	routineName := schemaRoutine[1]

	userSplit := strings.Split(grant.Principal.Id.Resource, ":")
	if len(userSplit) != 2 {
		return nil, fmt.Errorf("invalid principal ID: %s", grant.Principal.Id.Resource)
	}
	user := userSplit[1]

	err := s.client.RevokeRoutinePrivilege(ctx, privilege, schema, routineName, user)
	if err != nil {
		return nil, fmt.Errorf("failed to revoke %s on %s.%s from %s: %w", privilege, schema, routineName, user, err)
	}

	return nil, nil
}
