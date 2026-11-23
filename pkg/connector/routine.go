package connector

import (
	"context"
	"errors"
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

// routinePrivileges maps the entitlement privilege ID to the MySQL privilege name.
var routinePrivileges = map[string]string{
	"execute":       "EXECUTE",
	"alter_routine": "ALTER ROUTINE",
	"grant":         "GRANT OPTION",
}

// parseRoutineEntitlement splits "entitlement:<privilege>:routine:<schema>.<routine>"
// into the MySQL privilege name and the routine it applies to.
func parseRoutineEntitlement(entitlementID string) (string, string, string, error) {
	parts := strings.SplitN(entitlementID, ":", 4)
	if len(parts) != 4 {
		return "", "", "", fmt.Errorf("baton-mysql: invalid entitlement ID: %s", entitlementID)
	}

	if parts[2] != resourceTypeRoutine.Id {
		return "", "", "", fmt.Errorf("baton-mysql: unsupported resource kind in entitlement ID: %s", entitlementID)
	}

	privilege, ok := routinePrivileges[strings.ToLower(parts[1])]
	if !ok {
		return "", "", "", fmt.Errorf("baton-mysql: unsupported privilege for routine: %s", parts[1])
	}

	schema, routineName, found := strings.Cut(parts[3], ".")
	if !found || schema == "" || routineName == "" {
		return "", "", "", fmt.Errorf("baton-mysql: invalid routine name, expected schema.name: %s", parts[3])
	}

	return privilege, schema, routineName, nil
}

func (s *routineSyncer) routineAccount(principal *v2.Resource) (string, error) {
	switch principal.Id.ResourceType {
	case resourceTypeUser.Id:
	case resourceTypeRole.Id:
		if !s.client.IsVersion8() {
			return "", fmt.Errorf("baton-mysql: roles require MySQL 8")
		}
	default:
		return "", fmt.Errorf("baton-mysql: can only grant routine permissions to users and roles, got %s", principal.Id.ResourceType)
	}

	return principalAccount(principal.Id)
}

func (s *routineSyncer) Grant(ctx context.Context, principal *v2.Resource, entitlement *v2.Entitlement) (annotations.Annotations, error) {
	privilege, schema, routineName, err := parseRoutineEntitlement(entitlement.Id)
	if err != nil {
		return nil, err
	}

	user, err := s.routineAccount(principal)
	if err != nil {
		return nil, err
	}

	err = s.client.GrantRoutinePrivilege(ctx, privilege, schema, routineName, user)
	if err != nil {
		return nil, fmt.Errorf("baton-mysql: failed to grant %s on %s.%s to %s: %w", privilege, schema, routineName, user, err)
	}

	return nil, nil
}

func (s *routineSyncer) Revoke(ctx context.Context, grant *v2.Grant) (annotations.Annotations, error) {
	privilege, schema, routineName, err := parseRoutineEntitlement(grant.Entitlement.Id)
	if err != nil {
		return nil, err
	}

	user, err := s.routineAccount(grant.Principal)
	if err != nil {
		return nil, err
	}

	err = s.client.RevokeRoutinePrivilege(ctx, privilege, schema, routineName, user)
	switch {
	case errors.Is(err, client.ErrRoutineGrantNotFound):
		return annotations.New(&v2.GrantAlreadyRevoked{}), nil
	case err != nil:
		return nil, fmt.Errorf("baton-mysql: failed to revoke %s on %s.%s from %s: %w", privilege, schema, routineName, user, err)
	}

	return nil, nil
}
