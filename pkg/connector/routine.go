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
	"github.com/go-sql-driver/mysql"
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
	// Routine grants are emitted principal-side from listRoutineGrants in pkg/connector/grants.go.
	// The SDK writes grants additively via PutGrants (pkg/sync/ledger_grants.go in the SDK) over the
	// union of all syncer output, so leaving this syncer-side stub empty mirrors the established
	// empty stubs on tableSyncer, databaseSyncer, and columnSyncer.
	return nil, "", nil, nil
}

func newRoutineSyncer(c *client.Client) *routineSyncer {
	return &routineSyncer{
		resourceType: resourceTypeRoutine,
		client:       c,
	}
}
func (s *routineSyncer) Grant(ctx context.Context, principal *v2.Resource, entitlement *v2.Entitlement) (annotations.Annotations, error) {
	if !routinePrincipalAllowed(principal.Id.ResourceType, s.client.IsVersion8()) {
		return nil, fmt.Errorf("can only grant routine permissions to users (and roles on MySQL 8+)")
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

	privilege, err := routinePrivilege(rawPrivilege)
	if err != nil {
		return nil, err
	}

	schemaRoutine := strings.Split(fullRoutineName, ".")
	if len(schemaRoutine) != 2 {
		return nil, fmt.Errorf("invalid routine name, expected schema.name: %s", fullRoutineName)
	}
	schema := schemaRoutine[0]
	routineName := schemaRoutine[1]

	user, err := principalAccount(principal.Id)
	if err != nil {
		return nil, err
	}

	err = s.client.GrantRoutinePrivilege(ctx, privilege, schema, routineName, user)
	if err != nil {
		return nil, fmt.Errorf("failed to grant %s on %s.%s to %s: %w", privilege, schema, routineName, user, err)
	}

	return nil, nil
}

// routinePrivilege maps a raw routine entitlement privilege slug to the SQL privilege.
// Used only by the Grant path. The Revoke path never reaches this for `grant` (R1).
func routinePrivilege(rawPrivilege string) (string, error) {
	switch strings.ToLower(rawPrivilege) {
	case "execute":
		return "EXECUTE", nil
	case "alter_routine":
		return "ALTER ROUTINE", nil
	case "grant":
		return "GRANT OPTION", nil
	default:
		return "", fmt.Errorf("unsupported privilege for routine: %s", rawPrivilege)
	}
}

// routinePrincipalAllowed reports whether the principal resource type may receive routine grants.
// Users are always allowed; roles only on MySQL 8+ (I6).
func routinePrincipalAllowed(resourceType string, isVersion8 bool) bool {
	if resourceType == resourceTypeUser.Id {
		return true
	}
	return isVersion8 && resourceType == resourceTypeRole.Id
}

// isNoSuchGrantErr reports whether err is a MySQL "no such grant defined" error,
// which means the desired end state already holds. The error number is level-specific:
// 1147 (ER_NONEXISTING_TABLE_GRANT) is raised at the table level, while routine-level
// REVOKE raises 1403 (ER_NONEXISTING_PROC_GRANT, sql/auth/sql_authorization.cc
// mysql_routine_grant -> report_missing_user_grant_message). Both mean the same
// end-state, so both are treated as already-done.
func isNoSuchGrantErr(err error) bool {
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) {
		return mysqlErr.Number == 1147 || mysqlErr.Number == 1403
	}
	return false
}

// revokePerHosts runs revoke once per host of a (possibly collapsed) principal
// ("user@h1,h2"): MySQL 8 account-management statements are atomic, so a single
// REVOKE listing 'u'@'h1', 'u'@'h2' rolls back entirely when one host has no such
// grant (1403) while the other does. Per host, a no-such-grant error (1403/1147) is
// idempotent success for that host only; any other error is returned. DB-free so it
// is table-testable.
func revokePerHosts(user string, revoke func(account string) error) error {
	lookupUser, hosts, splitErr := client.SplitUserHost(user)
	if splitErr != nil {
		return fmt.Errorf("baton-mysql: malformed principal ID: %w", splitErr)
	}
	for _, host := range strings.Split(hosts, ",") {
		if err := revoke(lookupUser + "@" + host); err != nil {
			if isNoSuchGrantErr(err) {
				continue
			}
			return err
		}
	}
	return nil
}

// revokeRoutineGrantOption revokes the grant option on a routine WITHOUT revoking the
// held privileges themselves (R1): it builds
// `REVOKE GRANT OPTION ON <TYPE> <schema>.<routine> FROM <user>`
// via Client.RevokeRoutineGrantOption. MySQL's REVOKE grammar has no
// `GRANT OPTION FOR` clause (that is PostgreSQL; ER_PARSE_ERROR 1064); GRANT OPTION is
// a privilege of its own at the routine level, so this single statement clears only the
// Grant bit of the procs_priv row and leaves EXECUTE/ALTER ROUTINE in place. SQL runs
// even when nothing else is held: the Grant-only row is exactly the state
// GrantRoutinePrivilege's `GRANT USAGE ... WITH GRANT OPTION` fallback creates, and
// skipping it left the row in place so the next sync re-emitted the grant forever.
func (s *routineSyncer) revokeRoutineGrantOption(ctx context.Context, schema, routineName, user string) (annotations.Annotations, error) {
	if err := revokePerHosts(user, func(account string) error {
		return s.client.RevokeRoutineGrantOption(ctx, schema, routineName, account)
	}); err != nil {
		return nil, fmt.Errorf("failed to revoke grant option on %s.%s from %s: %w", schema, routineName, user, err)
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

	schemaRoutine := strings.Split(fullRoutineName, ".")
	if len(schemaRoutine) != 2 {
		return nil, fmt.Errorf("invalid routine name, expected schema.name: %s", fullRoutineName)
	}
	schema := schemaRoutine[0]
	routineName := schemaRoutine[1]

	user, err := principalAccount(grant.Principal.Id)
	if err != nil {
		return nil, err
	}

	// R1: revoke the grant option without revoking the held privileges. MySQL's REVOKE
	// grammar has no `GRANT OPTION FOR` clause (PostgreSQL syntax, ER_PARSE_ERROR 1064);
	// `REVOKE GRANT OPTION ON ...` is a privilege of its own and clears only the Grant bit
	// of the procs_priv row, leaving EXECUTE/ALTER ROUTINE in place.
	if strings.ToLower(rawPrivilege) == "grant" {
		return s.revokeRoutineGrantOption(ctx, schema, routineName, user)
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

	// The revoke runs once per host of a (possibly collapsed) principal — see
	// revokePerHosts for why the statement must not list both hosts at once.
	if err := revokePerHosts(user, func(account string) error {
		return s.client.RevokeRoutinePrivilege(ctx, privilege, schema, routineName, account)
	}); err != nil {
		return nil, fmt.Errorf("failed to revoke %s on %s.%s from %s: %w", privilege, schema, routineName, user, err)
	}

	return nil, nil
}
