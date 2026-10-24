package connector

import (
	"context"
	"fmt"
	"strings"

	"github.com/conductorone/baton-mysql/pkg/client"
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/pagination"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
	keyword, withGrant, err := serverPrivilege(entitlement.Id)
	if err != nil {
		return nil, err
	}

	userStr, err := principalAccount(principal.Id)
	if err != nil {
		return nil, err
	}

	err = s.client.GrantServerPrivilege(ctx, userStr, keyword, withGrant)
	if err != nil {
		return nil, fmt.Errorf("grant failed: %w", err)
	}

	return nil, nil
}

func (s *serverSyncer) Revoke(ctx context.Context, grant *v2.Grant) (annotations.Annotations, error) {
	keyword, _, err := serverPrivilege(grant.Entitlement.Id)
	if err != nil {
		return nil, err
	}

	userStr, err := principalAccount(grant.Principal.Id)
	if err != nil {
		return nil, err
	}

	err = s.client.RevokeServerPrivilege(ctx, userStr, keyword)
	if err != nil {
		return nil, fmt.Errorf("revoke failed: %w", err)
	}

	return nil, nil
}

// staticServerPrivileges maps the catalog keys whose MySQL keyword differs
// from the blanket strings.ToUpper: MySQL 8 multi-word static privileges, which
// are space-separated, and the dynamic privilege nbd_stored_user, whose MySQL
// acronym is NDB_STORED_USER. Every other key (single-word static privileges
// and the remaining dynamic privileges, e.g. backup_admin -> BACKUP_ADMIN)
// falls through to strings.ToUpper.
var staticServerPrivileges = map[string]string{
	"alter_routine":           "ALTER ROUTINE",
	"create_role":             "CREATE ROLE",
	"create_routine":          "CREATE ROUTINE",
	"create_tablespace":       "CREATE TABLESPACE",
	"create_temporary_tables": "CREATE TEMPORARY TABLES",
	"create_user":             "CREATE USER",
	"create_view":             "CREATE VIEW",
	"drop_role":               "DROP ROLE",
	"grant":                   "GRANT OPTION",
	"lock_tables":             "LOCK TABLES",
	"nbd_stored_user":         "NDB_STORED_USER",
	"replication_client":      "REPLICATION CLIENT",
	"replication_slave":       "REPLICATION SLAVE",
	"show_databases":          "SHOW DATABASES",
	"show_view":               "SHOW VIEW",
}

// reservedServerKeys are catalog keys that reach server scope but are not
// privilege keywords. They keep their pre-fix emission byte-for-byte.
var reservedServerKeys = map[string]struct{}{
	"proxy":            {},
	"proxy_with_grant": {},
}

// legacyServerPrivilege reproduces the pre-fix server-scope spelling for the
// reserved keys.
func legacyServerPrivilege(key string) string {
	priv := strings.ToUpper(strings.ReplaceAll(key, "_WITH_GRANT", ""))
	if strings.Contains(key, "_with_grant") {
		return priv + " WITH GRANT OPTION"
	}
	return priv
}

// serverPrivilege translates a server-scope entitlement ID into the privilege
// keyword MySQL expects and whether the statement must append WITH GRANT OPTION.
func serverPrivilege(entitlementID string) (string, bool, error) {
	parts := strings.SplitN(entitlementID, ":", 3)
	if len(parts) != 3 || parts[1] == "" {
		return "", false, status.Errorf(codes.InvalidArgument, "baton-mysql: invalid entitlement ID: %s", entitlementID)
	}

	key := parts[1]
	if _, ok := reservedServerKeys[key]; ok {
		return legacyServerPrivilege(key), false, nil
	}

	withGrant := false
	if strings.HasSuffix(key, "_with_grant") {
		withGrant = true
		key = strings.TrimSuffix(key, "_with_grant")
	}

	if kw, ok := staticServerPrivileges[key]; ok {
		return kw, withGrant, nil
	}

	return strings.ToUpper(key), withGrant, nil
}
