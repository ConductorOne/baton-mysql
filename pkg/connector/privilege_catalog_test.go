package connector

import (
	"strings"
	"testing"

	"github.com/conductorone/baton-mysql/pkg/client"
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
)

// showPrivileges is the privilege list a stock MySQL 8.0.46 registers, captured from
// SHOW PRIVILEGES, so the published catalog can be checked without a server.
var showPrivileges = []string{
	"Alter",
	"Alter routine",
	"APPLICATION_PASSWORD_ADMIN",
	"AUDIT_ABORT_EXEMPT",
	"AUDIT_ADMIN",
	"AUTHENTICATION_POLICY_ADMIN",
	"BACKUP_ADMIN",
	"BINLOG_ADMIN",
	"BINLOG_ENCRYPTION_ADMIN",
	"CLONE_ADMIN",
	"CONNECTION_ADMIN",
	"Create",
	"Create role",
	"Create routine",
	"Create tablespace",
	"Create temporary tables",
	"Create user",
	"Create view",
	"Delete",
	"Drop",
	"Drop role",
	"ENCRYPTION_KEY_ADMIN",
	"Event",
	"Execute",
	"File",
	"FIREWALL_EXEMPT",
	"FLUSH_OPTIMIZER_COSTS",
	"FLUSH_STATUS",
	"FLUSH_TABLES",
	"FLUSH_USER_RESOURCES",
	"Grant option",
	"GROUP_REPLICATION_ADMIN",
	"GROUP_REPLICATION_STREAM",
	"Index",
	"INNODB_REDO_LOG_ARCHIVE",
	"INNODB_REDO_LOG_ENABLE",
	"Insert",
	"Lock tables",
	"PASSWORDLESS_USER_ADMIN",
	"PERSIST_RO_VARIABLES_ADMIN",
	"Process",
	"Proxy",
	"References",
	"Reload",
	"REPLICATION_APPLIER",
	"Replication client",
	"Replication slave",
	"REPLICATION_SLAVE_ADMIN",
	"RESOURCE_GROUP_ADMIN",
	"RESOURCE_GROUP_USER",
	"ROLE_ADMIN",
	"Select",
	"SENSITIVE_VARIABLES_OBSERVER",
	"SERVICE_CONNECTION_ADMIN",
	"SESSION_VARIABLES_ADMIN",
	"SET_USER_ID",
	"Show databases",
	"SHOW_ROUTINE",
	"Show view",
	"Shutdown",
	"Super",
	"SYSTEM_USER",
	"SYSTEM_VARIABLES_ADMIN",
	"TABLE_ENCRYPTION_ADMIN",
	"TELEMETRY_LOG_ADMIN",
	"Trigger",
	"Update",
	"Usage",
	"XA_RECOVER_ADMIN",
}

// unregisteredOnStockServer are published slugs a stock server does not register,
// because the privilege arrives with an optional plugin or an NDB Cluster build. Grant
// and revoke for them fail on a server without it, which is the server answering, not a
// defect in the catalog. nbd_stored_user is also misspelled and needs the alias
// MatchPrivilege applies; TestMatchPrivilege covers that half, since a server that does
// register NDB_STORED_USER cannot be a fixture here.
var unregisteredOnStockServer = map[string]struct{}{
	"firewall_admin":      {},
	"firewall_user":       {},
	"nbd_stored_user":     {},
	"skip_query_rewrite":  {},
	"version_token_admin": {},
}

// Every privilege slug the connector publishes on a server, database or table has to
// name a privilege the server registers, because grant and revoke resolve the slug
// against SHOW PRIVILEGES before building the statement. A slug that does not resolve is
// an entitlement ConductorOne offers and the connector can never fulfil, and the slug is
// part of the entitlement ID, so correcting it later would re-key every grant holding it.
func TestPublishedPrivilegeSlugsResolve(t *testing.T) {
	for _, rt := range []*v2.ResourceType{resourceTypeServer, resourceTypeDatabase, resourceTypeTable} {
		for _, tmpl := range entitlementsByResourceType[rt.Id] {
			base, _ := strings.CutSuffix(tmpl.ID, "_with_grant")
			if _, ok := unregisteredOnStockServer[base]; ok {
				continue
			}
			if base == proxyPriv {
				// Proxy is granted on an account, never on a scope, and the client
				// rejects it here on purpose.
				continue
			}

			t.Run(rt.Id+"/"+tmpl.ID, func(t *testing.T) {
				if _, _, err := client.MatchPrivilege(tmpl.ID, showPrivileges); err != nil {
					t.Error(err)
				}
			})
		}
	}
}
