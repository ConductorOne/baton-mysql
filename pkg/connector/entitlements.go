package connector

import (
	"context"
	"fmt"
	"strings"

	"github.com/conductorone/baton-mysql/pkg/client"
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
)

const (
	proxyWithGrantPriv          = "proxy_with_grant"
	proxyPriv                   = "proxy"
	roleAssignmentPriv          = "role_assignment"
	roleAssignmentWithGrantPriv = "role_assignment_with_grant"
)

type entitlementTemplate struct {
	ID               string
	includeWithGrant bool
	resourceTypes    []*v2.ResourceType
	entitlement      v2.Entitlement
	v8Only           bool
}

func getEntitlementsForResource(ctx context.Context, resource *v2.Resource, c *client.Client) ([]*v2.Entitlement, error) {
	tmpls, ok := entitlementsByResourceType[resource.Id.ResourceType]
	if !ok || len(tmpls) == 0 {
		return nil, nil
	}

	grantable := []*v2.ResourceType{resourceTypeUser}
	if c.IsVersion8() {
		grantable = append(grantable, resourceTypeRole)
	}

	var ret []*v2.Entitlement
	for _, t := range tmpls {
		if !c.IsVersion8() && t.v8Only {
			continue
		}
		dName := getEntitlementDisplayName(ctx, t, resource)
		dDescription := getEntitlementDescription(ctx, t, resource)
		ret = append(ret, &v2.Entitlement{
			Resource:    resource,
			Id:          fmt.Sprintf("entitlement:%s:%s", t.ID, resource.Id.Resource),
			DisplayName: dName,
			Description: dDescription,
			GrantableTo: grantable,
			Annotations: t.entitlement.Annotations,
			Purpose:     t.entitlement.Purpose,
			Slug:        strings.ToLower(getEntitlementSlug(ctx, t, resource)),
		})
	}

	return ret, nil
}

func getEntitlementSlug(ctx context.Context, e *entitlementTemplate, resource *v2.Resource) string {
	switch resource.Id.ResourceType {
	case resourceTypeRole.Id, resourceTypeUser.Id:
		switch e.ID {
		case proxyWithGrantPriv:
			return "grant proxy"
		case roleAssignmentWithGrantPriv:
			return "grant role"
		case proxyPriv:
			return "proxy"
		case roleAssignmentPriv:
			return "member"
		default:
			return e.entitlement.DisplayName
		}

	default:
		return e.entitlement.DisplayName
	}
}

func getEntitlementDisplayName(ctx context.Context, e *entitlementTemplate, resource *v2.Resource) string {
	rID := strings.TrimPrefix(resource.Id.Resource, fmt.Sprintf("%s:", resource.Id.ResourceType))
	upperDisplayName := strings.ToUpper(e.entitlement.DisplayName)
	switch resource.Id.ResourceType {
	case resourceTypeServer.Id:
		return fmt.Sprintf("%s *.*", upperDisplayName)
	case resourceTypeDatabase.Id:
		return fmt.Sprintf("%s %s.*", upperDisplayName, rID)
	case resourceTypeTable.Id, resourceTypeRoutine.Id:
		return fmt.Sprintf("%s %s", upperDisplayName, rID)
	case resourceTypeColumn.Id:
		rParts := strings.Split(rID, ".")
		return fmt.Sprintf("%s (%s) %s %s", strings.ToUpper(e.entitlement.DisplayName), rParts[len(rParts)-1], titleCaser.String(resource.Id.ResourceType), rID)
	case resourceTypeRole.Id, resourceTypeUser.Id:
		switch e.ID {
		case "proxy_with_grant":
			return fmt.Sprintf("GRANT PROXY %s", rID)
		case "role_assignment_with_grant":
			return fmt.Sprintf("GRANT ROLE %s", rID)
		case "proxy":
			return fmt.Sprintf("%s %s", upperDisplayName, rID)
		case "role_assignment":
			return fmt.Sprintf("%s Role Member", rID)
		}
		// This is a grant priv
		if strings.Contains(e.ID, "_with_grant") {
			return fmt.Sprintf("GRANT %s", rID)
		}
		return fmt.Sprintf("%s %s", rID, e.entitlement.DisplayName)
	}

	return e.entitlement.DisplayName
}

func getEntitlementDescription(ctx context.Context, e *entitlementTemplate, resource *v2.Resource) string {
	rID := strings.TrimPrefix(resource.Id.Resource, fmt.Sprintf("%s:", resource.Id.ResourceType))
	switch resource.Id.ResourceType {
	case resourceTypeServer.Id:
		return fmt.Sprintf("%s globally", e.entitlement.Description)
	case resourceTypeDatabase.Id:
		return fmt.Sprintf("%s on the %s database", e.entitlement.Description, rID)
	case resourceTypeTable.Id:
		return fmt.Sprintf("%s on the %s table", e.entitlement.Description, rID)
	case resourceTypeRoutine.Id:
		return fmt.Sprintf("%s on the %s routine", e.entitlement.Description, rID)
	case resourceTypeColumn.Id:
		rParts := strings.Split(rID, ".")
		return fmt.Sprintf("%s on the %s column on the %s table", e.entitlement.Description, rParts[len(rParts)-1], rID)
	case resourceTypeRole.Id, resourceTypeUser.Id:
		switch e.ID {
		case proxyWithGrantPriv:
			return fmt.Sprintf("Allows granting other users the ability to proxy to the %s user", rID)
		case roleAssignmentWithGrantPriv:
			return fmt.Sprintf("Allows granting other users the ability to use SET ROLE with the %s role", rID)
		case proxyPriv:
			return fmt.Sprintf("Enables proxying to the %s user", rID)
		case roleAssignmentPriv:
			return fmt.Sprintf("%s on the %s role", e.entitlement.Description, rID)
		}
	}

	return e.entitlement.Description
}

var entitlementsByResourceType map[string][]*entitlementTemplate

//nolint:gochecknoinits // Init for creating all entitlement objects
func init() {
	entitlementsByResourceType = make(map[string][]*entitlementTemplate)
	for _, rt := range allResourceTypes {
		entitlementsByResourceType[rt.Id] = []*entitlementTemplate{}
	}
	// This is a map of all the entitlements available in MySQL 8
	// Source: https://dev.mysql.com/doc/refman/8.0/en/grant.html
	allEntitlements := map[string]*entitlementTemplate{
		"alter": {
			resourceTypes: globalDatabaseTableScope,
			entitlement: v2.Entitlement{
				DisplayName: "Alter",
				Description: "Enable use of ALTER TABLE",
				Annotations: nil,
				Purpose:     v2.Entitlement_PURPOSE_VALUE_PERMISSION,
			},
		},
		"alter_routine": {
			resourceTypes: append(globalDatabaseScope, resourceTypeRoutine),
			entitlement: v2.Entitlement{
				DisplayName: "Alter routine",
				Description: "Enable stored routines to be altered or dropped",
				Annotations: nil,
				Purpose:     v2.Entitlement_PURPOSE_VALUE_PERMISSION,
			},
		},
		"create": {
			resourceTypes: globalDatabaseTableScope,
			entitlement: v2.Entitlement{
				DisplayName: "Create",
				Description: "Enable database and table creation",
				Annotations: nil,
				Purpose:     v2.Entitlement_PURPOSE_VALUE_PERMISSION,
			},
		},
		"create_role": {
			v8Only:        true,
			resourceTypes: globalScope,
			entitlement: v2.Entitlement{
				DisplayName: "Create role",
				Description: "Enable role creation",
				Annotations: nil,
				Purpose:     v2.Entitlement_PURPOSE_VALUE_PERMISSION,
			},
		},
		"create_routine": {
			resourceTypes: globalDatabaseScope,
			entitlement: v2.Entitlement{
				DisplayName: "Create routine",
				Description: "Enable stored routine creation",
				Annotations: nil,
				Purpose:     v2.Entitlement_PURPOSE_VALUE_PERMISSION,
			},
		},
		"create_tablespace": {
			resourceTypes: globalScope,
			entitlement: v2.Entitlement{
				DisplayName: "Create tablespace",
				Description: "Enable tablespaces and log file groups to be created, altered, or dropped",
				Annotations: nil,
				Purpose:     v2.Entitlement_PURPOSE_VALUE_PERMISSION,
			},
		},
		"create_temporary_tables": {
			resourceTypes: globalDatabaseScope,
			entitlement: v2.Entitlement{
				DisplayName: "Create temporary tables",
				Description: "Enable use of CREATE TEMPORARY TABLE",
				Annotations: nil,
				Purpose:     v2.Entitlement_PURPOSE_VALUE_PERMISSION,
			},
		},
		"create_user": {
			resourceTypes: globalScope,
			entitlement: v2.Entitlement{
				DisplayName: "Create user",
				Description: "Enable use of CREATE USER, DROP USER, RENAME USER, and REVOKE ALL PRIVILEGES",
				Annotations: nil,
				Purpose:     v2.Entitlement_PURPOSE_VALUE_PERMISSION,
			},
		},
		"create_view": {
			resourceTypes: globalDatabaseTableScope,
			entitlement: v2.Entitlement{
				DisplayName: "Create view",
				Description: "Enable views to be created or altered",
				Annotations: nil,
				Purpose:     v2.Entitlement_PURPOSE_VALUE_PERMISSION,
			},
		},
		"delete": {
			resourceTypes: globalDatabaseTableScope,
			entitlement: v2.Entitlement{
				DisplayName: "Delete",
				Description: "Enable use of DELETE",
				Annotations: nil,
				Purpose:     v2.Entitlement_PURPOSE_VALUE_PERMISSION,
			},
		},
		"drop": {
			resourceTypes: globalDatabaseTableScope,
			entitlement: v2.Entitlement{
				DisplayName: "Drop",
				Description: "Enable databases, tables, and views to be dropped",
				Annotations: nil,
				Purpose:     v2.Entitlement_PURPOSE_VALUE_PERMISSION,
			},
		},
		"drop_role": {
			v8Only:        true,
			resourceTypes: globalScope,
			entitlement: v2.Entitlement{
				DisplayName: "Drop role",
				Description: "Enable roles to be dropped",
				Annotations: nil,
				Purpose:     v2.Entitlement_PURPOSE_VALUE_PERMISSION,
			},
		},
		"event": {
			resourceTypes: globalDatabaseScope,
			entitlement: v2.Entitlement{
				DisplayName: "Event",
				Description: "Enable use of events for the Event Scheduler",
				Annotations: nil,
				Purpose:     v2.Entitlement_PURPOSE_VALUE_PERMISSION,
			},
		},
		"execute": {
			resourceTypes: append(globalDatabaseScope, resourceTypeRoutine),
			entitlement: v2.Entitlement{
				DisplayName: "Execute",
				Description: "Enable the user to execute stored routines",
				Annotations: nil,
				Purpose:     v2.Entitlement_PURPOSE_VALUE_PERMISSION,
			},
		},
		"file": {
			resourceTypes: globalScope,
			entitlement: v2.Entitlement{
				DisplayName: "File",
				Description: "Enable the user to cause the server to read or write files",
				Annotations: nil,
				Purpose:     v2.Entitlement_PURPOSE_VALUE_PERMISSION,
			},
		},
		"grant": {
			resourceTypes: append(globalDatabaseTableColumnScope, resourceTypeRoutine),
			entitlement: v2.Entitlement{
				DisplayName: "Grant",
				Description: "Enable privileges to be granted to or removed from other accounts",
				Annotations: nil,
				Purpose:     v2.Entitlement_PURPOSE_VALUE_PERMISSION,
			},
		},
		"index": {
			resourceTypes: globalDatabaseTableScope,
			entitlement: v2.Entitlement{
				DisplayName: "Index",
				Description: "Enable indexes to be created or dropped",
				Annotations: nil,
				Purpose:     v2.Entitlement_PURPOSE_VALUE_PERMISSION,
			},
		},
		"insert": {
			resourceTypes: globalDatabaseTableColumnScope,
			entitlement: v2.Entitlement{
				DisplayName: "Insert",
				Description: "Enable use of INSERT",
				Annotations: nil,
				Purpose:     v2.Entitlement_PURPOSE_VALUE_PERMISSION,
			},
		},
		"lock_tables": {
			resourceTypes: globalDatabaseScope,
			entitlement: v2.Entitlement{
				DisplayName: "Lock tables",
				Description: "Enable use of LOCK TABLES on tables for which you have the SELECT privilege",
				Annotations: nil,
				Purpose:     v2.Entitlement_PURPOSE_VALUE_PERMISSION,
			},
		},
		"process": {
			resourceTypes: globalScope,
			entitlement: v2.Entitlement{
				DisplayName: "Process",
				Description: "Enable the user to see all processes with SHOW PROCESSLIST",
				Annotations: nil,
				Purpose:     v2.Entitlement_PURPOSE_VALUE_PERMISSION,
			},
		},
		"proxy": {
			resourceTypes:    []*v2.ResourceType{resourceTypeServer, resourceTypeUser, resourceTypeRole},
			includeWithGrant: true,
			entitlement: v2.Entitlement{
				DisplayName: "Proxy",
				Description: "Enable user proxying",
				Annotations: nil,
				Purpose:     v2.Entitlement_PURPOSE_VALUE_PERMISSION,
			},
		},
		"references": {
			resourceTypes: globalDatabaseTableColumnScope,
			entitlement: v2.Entitlement{
				DisplayName: "References",
				Description: "Enable foreign key creation",
				Annotations: nil,
				Purpose:     v2.Entitlement_PURPOSE_VALUE_PERMISSION,
			},
		},
		"reload": {
			resourceTypes: globalScope,
			entitlement: v2.Entitlement{
				DisplayName: "Reload",
				Description: "Enable use of FLUSH operations",
				Annotations: nil,
				Purpose:     v2.Entitlement_PURPOSE_VALUE_PERMISSION,
			},
		},
		"replication_client": {
			resourceTypes: globalScope,
			entitlement: v2.Entitlement{
				DisplayName: "Replication client",
				Description: "Enable the user to ask where source or replica servers are.",
				Annotations: nil,
				Purpose:     v2.Entitlement_PURPOSE_VALUE_PERMISSION,
			},
		},
		"replication_slave": {
			resourceTypes: globalScope,
			entitlement: v2.Entitlement{
				DisplayName: "Replication slave",
				Description: "Enable replicas to read binary log events from the source",
				Annotations: nil,
				Purpose:     v2.Entitlement_PURPOSE_VALUE_PERMISSION,
			},
		},
		"select": {
			resourceTypes: globalDatabaseTableColumnScope,
			entitlement: v2.Entitlement{
				DisplayName: "Select",
				Description: "Enable use of SELECT",
				Annotations: nil,
				Purpose:     v2.Entitlement_PURPOSE_VALUE_PERMISSION,
			},
		},
		"show_databases": {
			resourceTypes: globalScope,
			entitlement: v2.Entitlement{
				DisplayName: "Show databases",
				Description: "Enable SHOW DATABASES to show all databases.",
				Annotations: nil,
				Purpose:     v2.Entitlement_PURPOSE_VALUE_PERMISSION,
			},
		},
		"show_view": {
			resourceTypes: globalDatabaseTableScope,
			entitlement: v2.Entitlement{
				DisplayName: "Show view",
				Description: "Enable use of SHOW CREATE VIEW",
				Annotations: nil,
				Purpose:     v2.Entitlement_PURPOSE_VALUE_PERMISSION,
			},
		},
		"shutdown": {
			resourceTypes: globalScope,
			entitlement: v2.Entitlement{
				DisplayName: "Shutdown",
				Description: "Enable use of mysqladmin shutdown",
				Annotations: nil,
				Purpose:     v2.Entitlement_PURPOSE_VALUE_PERMISSION,
			},
		},
		"super": {
			resourceTypes: globalScope,
			entitlement: v2.Entitlement{
				DisplayName: "Super",
				Description: "Enable use of other administrative operations such as CHANGE REPLICATION SOURCE TO, CHANGE MASTER TO, " +
					"KILL, PURGE BINARY LOGS, SET GLOBAL, and mysqladmin debug command.",
				Annotations: nil,
				Purpose:     v2.Entitlement_PURPOSE_VALUE_PERMISSION,
			},
		},
		"trigger": {
			resourceTypes: globalDatabaseTableScope,
			entitlement: v2.Entitlement{
				DisplayName: "Trigger",
				Description: "Enable trigger operations",
				Annotations: nil,
				Purpose:     v2.Entitlement_PURPOSE_VALUE_PERMISSION,
			},
		},
		"update": {
			resourceTypes: globalDatabaseTableColumnScope,
			entitlement: v2.Entitlement{
				DisplayName: "Update",
				Description: "Enable use of UPDATE",
				Annotations: nil,
				Purpose:     v2.Entitlement_PURPOSE_VALUE_PERMISSION,
			},
		},
		"application_password_admin": {
			resourceTypes:    globalScope,
			v8Only:           true,
			includeWithGrant: true,
			entitlement: v2.Entitlement{
				DisplayName: "Application password admin",
				Description: "Enable dual password administration",
				Annotations: nil,
				Purpose:     v2.Entitlement_PURPOSE_VALUE_PERMISSION,
			},
		},
		"audit_abort_exempt": {
			resourceTypes:    globalScope,
			v8Only:           true,
			includeWithGrant: true,
			entitlement: v2.Entitlement{
				DisplayName: "Audit abort exempt",
				Description: "Allow queries blocked by audit log filter",
				Annotations: nil,
				Purpose:     v2.Entitlement_PURPOSE_VALUE_PERMISSION,
			},
		},
		"audit_admin": {
			resourceTypes:    globalScope,
			v8Only:           true,
			includeWithGrant: true,
			entitlement: v2.Entitlement{
				DisplayName: "Audit admin",
				Description: "Enable audit log configuration",
				Annotations: nil,
				Purpose:     v2.Entitlement_PURPOSE_VALUE_PERMISSION,
			},
		},
		"authentication_policy_admin": {
			resourceTypes:    globalScope,
			v8Only:           true,
			includeWithGrant: true,
			entitlement: v2.Entitlement{
				DisplayName: "Authentication policy admin",
				Description: "Enable authentication policy administration.",
				Annotations: nil,
				Purpose:     v2.Entitlement_PURPOSE_VALUE_PERMISSION,
			},
		},
		"backup_admin": {
			resourceTypes:    globalScope,
			v8Only:           true,
			includeWithGrant: true,
			entitlement: v2.Entitlement{
				DisplayName: "Backup admin",
				Description: "Enable backup administration",
				Annotations: nil,
				Purpose:     v2.Entitlement_PURPOSE_VALUE_PERMISSION,
			},
		},
		"binlog_admin": {
			resourceTypes:    globalScope,
			v8Only:           true,
			includeWithGrant: true,
			entitlement: v2.Entitlement{
				DisplayName: "Binlog admin",
				Description: "Enable binary log control",
				Annotations: nil,
				Purpose:     v2.Entitlement_PURPOSE_VALUE_PERMISSION,
			},
		},
		"binlog_encryption_admin": {
			resourceTypes:    globalScope,
			v8Only:           true,
			includeWithGrant: true,
			entitlement: v2.Entitlement{
				DisplayName: "Binlog encryption admin",
				Description: "Enable activation and deactivation of binary log encryption",
				Annotations: nil,
				Purpose:     v2.Entitlement_PURPOSE_VALUE_PERMISSION,
			},
		},
		"clone_admin": {
			resourceTypes:    globalScope,
			v8Only:           true,
			includeWithGrant: true,
			entitlement: v2.Entitlement{
				DisplayName: "Clone admin",
				Description: "Enable clone administration",
				Annotations: nil,
				Purpose:     v2.Entitlement_PURPOSE_VALUE_PERMISSION,
			},
		},
		"connection_admin": {
			resourceTypes:    globalScope,
			v8Only:           true,
			includeWithGrant: true,
			entitlement: v2.Entitlement{
				DisplayName: "Connection admin",
				Description: "Enable connection limit/restriction control",
				Annotations: nil,
				Purpose:     v2.Entitlement_PURPOSE_VALUE_PERMISSION,
			},
		},
		"encryption_key_admin": {
			resourceTypes:    globalScope,
			v8Only:           true,
			includeWithGrant: true,
			entitlement: v2.Entitlement{
				DisplayName: "Encryption key admin",
				Description: "Enable InnoDB key rotation",
				Annotations: nil,
				Purpose:     v2.Entitlement_PURPOSE_VALUE_PERMISSION,
			},
		},
		"firewall_admin": {
			resourceTypes:    globalScope,
			v8Only:           true,
			includeWithGrant: true,
			entitlement: v2.Entitlement{
				DisplayName: "Firewall admin",
				Description: "Enable firewall rule administration, any user",
				Annotations: nil,
				Purpose:     v2.Entitlement_PURPOSE_VALUE_PERMISSION,
			},
		},
		"firewall_exempt": {
			resourceTypes:    globalScope,
			v8Only:           true,
			includeWithGrant: true,
			entitlement: v2.Entitlement{
				DisplayName: "Firewall exempt",
				Description: "Exempt user from firewall restrictions",
				Annotations: nil,
				Purpose:     v2.Entitlement_PURPOSE_VALUE_PERMISSION,
			},
		},
		"firewall_user": {
			resourceTypes:    globalScope,
			v8Only:           true,
			includeWithGrant: true,
			entitlement: v2.Entitlement{
				DisplayName: "Firewall user",
				Description: "Enable firewall rule administration, self",
				Annotations: nil,
				Purpose:     v2.Entitlement_PURPOSE_VALUE_PERMISSION,
			},
		},
		"flush_optimizer_costs": {
			resourceTypes:    globalScope,
			v8Only:           true,
			includeWithGrant: true,
			entitlement: v2.Entitlement{
				DisplayName: "Flush optimizer costs",
				Description: "Enable optimizer cost reloading",
				Annotations: nil,
				Purpose:     v2.Entitlement_PURPOSE_VALUE_PERMISSION,
			},
		},
		"flush_status": {
			resourceTypes:    globalScope,
			v8Only:           true,
			includeWithGrant: true,
			entitlement: v2.Entitlement{
				DisplayName: "Flush status",
				Description: "Enable status indicator flushing",
				Annotations: nil,
				Purpose:     v2.Entitlement_PURPOSE_VALUE_PERMISSION,
			},
		},
		"flush_tables": {
			resourceTypes:    globalScope,
			v8Only:           true,
			includeWithGrant: true,
			entitlement: v2.Entitlement{
				DisplayName: "Flush tables",
				Description: "Enable table flushing",
				Annotations: nil,
				Purpose:     v2.Entitlement_PURPOSE_VALUE_PERMISSION,
			},
		},
		"flush_user_resources": {
			resourceTypes:    globalScope,
			v8Only:           true,
			includeWithGrant: true,
			entitlement: v2.Entitlement{
				DisplayName: "Flush user resources",
				Description: "Enable user-resource flushing",
				Annotations: nil,
				Purpose:     v2.Entitlement_PURPOSE_VALUE_PERMISSION,
			},
		},
		"group_replication_admin": {
			resourceTypes:    globalScope,
			v8Only:           true,
			includeWithGrant: true,
			entitlement: v2.Entitlement{
				DisplayName: "Group replication admin",
				Description: "Enable Group Replication control",
				Annotations: nil,
				Purpose:     v2.Entitlement_PURPOSE_VALUE_PERMISSION,
			},
		},
		"group_replication_stream": {
			resourceTypes:    globalScope,
			v8Only:           true,
			includeWithGrant: true,
			entitlement: v2.Entitlement{
				DisplayName: "Group replication stream",
				Description: "Allows a user account to be used for establishing Group Replication's group communication connections.",
				Annotations: nil,
				Purpose:     v2.Entitlement_PURPOSE_VALUE_PERMISSION,
			},
		},
		"innodb_redo_log_enable": {
			resourceTypes:    globalScope,
			v8Only:           true,
			includeWithGrant: true,
			entitlement: v2.Entitlement{
				DisplayName: "InnoDB redo log enable",
				Description: "Enable or disable redo logging",
				Annotations: nil,
				Purpose:     v2.Entitlement_PURPOSE_VALUE_PERMISSION,
			},
		},
		"innodb_redo_log_archive": {
			resourceTypes:    globalScope,
			v8Only:           true,
			includeWithGrant: true,
			entitlement: v2.Entitlement{
				DisplayName: "InnoDB redo log archive",
				Description: "Enable redo log archiving administration",
				Annotations: nil,
				Purpose:     v2.Entitlement_PURPOSE_VALUE_PERMISSION,
			},
		},
		"nbd_stored_user": {
			resourceTypes:    globalScope,
			v8Only:           true,
			includeWithGrant: true,
			entitlement: v2.Entitlement{
				DisplayName: "NDB stored user",
				Description: "Enable sharing of user or role between SQL nodes (NDB Cluster)",
				Annotations: nil,
				Purpose:     v2.Entitlement_PURPOSE_VALUE_PERMISSION,
			},
		},
		"passwordless_user_admin": {
			resourceTypes:    globalScope,
			v8Only:           true,
			includeWithGrant: true,
			entitlement: v2.Entitlement{
				DisplayName: "Passwordless user admin",
				Description: "Enable passwordless user account administration",
				Annotations: nil,
				Purpose:     v2.Entitlement_PURPOSE_VALUE_PERMISSION,
			},
		},
		"persist_ro_variables_admin": {
			resourceTypes:    globalScope,
			v8Only:           true,
			includeWithGrant: true,
			entitlement: v2.Entitlement{
				DisplayName: "Persist RO variables admin",
				Description: "Enable persisting read-only system variables",
				Annotations: nil,
				Purpose:     v2.Entitlement_PURPOSE_VALUE_PERMISSION,
			},
		},
		"replication_applier": {
			resourceTypes:    globalScope,
			v8Only:           true,
			includeWithGrant: true,
			entitlement: v2.Entitlement{
				DisplayName: "Replication applier",
				Description: "Act as the PRIVILEGE_CHECKS_USER for a replication channel",
				Annotations: nil,
				Purpose:     v2.Entitlement_PURPOSE_VALUE_PERMISSION,
			},
		},
		"replication_slave_admin": {
			resourceTypes:    globalScope,
			v8Only:           true,
			includeWithGrant: true,
			entitlement: v2.Entitlement{
				DisplayName: "Replication slave admin",
				Description: "Enable regular replication control",
				Annotations: nil,
				Purpose:     v2.Entitlement_PURPOSE_VALUE_PERMISSION,
			},
		},
		"resource_group_admin": {
			resourceTypes:    globalScope,
			v8Only:           true,
			includeWithGrant: true,
			entitlement: v2.Entitlement{
				DisplayName: "Resource group admin",
				Description: "Enable resource group administration",
				Annotations: nil,
				Purpose:     v2.Entitlement_PURPOSE_VALUE_PERMISSION,
			},
		},
		"resource_group_user": {
			resourceTypes:    globalScope,
			v8Only:           true,
			includeWithGrant: true,
			entitlement: v2.Entitlement{
				DisplayName: "Resource group user",
				Description: "Enable resource group administration",
				Annotations: nil,
				Purpose:     v2.Entitlement_PURPOSE_VALUE_PERMISSION,
			},
		},
		"role_admin": {
			v8Only:           true,
			resourceTypes:    globalScope,
			includeWithGrant: true,
			entitlement: v2.Entitlement{
				DisplayName: "Role admin",
				Description: "Enable roles to be granted or revoked, use of WITH ADMIN OPTION",
				Annotations: nil,
				Purpose:     v2.Entitlement_PURPOSE_VALUE_PERMISSION,
			},
		},
		"sensitive_variables_observer": {
			resourceTypes:    globalScope,
			v8Only:           true,
			includeWithGrant: true,
			entitlement: v2.Entitlement{
				DisplayName: "Sensitive variables observer",
				Description: "Enables connections to the network interface that permits only administrative connections",
				Annotations: nil,
				Purpose:     v2.Entitlement_PURPOSE_VALUE_PERMISSION,
			},
		},
		"service_connection_admin": {
			resourceTypes:    globalScope,
			v8Only:           true,
			includeWithGrant: true,
			entitlement: v2.Entitlement{
				DisplayName: "Service connection admin",
				Description: "Enables connections to the network interface that permits only administrative connections",
				Annotations: nil,
				Purpose:     v2.Entitlement_PURPOSE_VALUE_PERMISSION,
			},
		},
		"session_variables_admin": {
			resourceTypes:    globalScope,
			v8Only:           true,
			includeWithGrant: true,
			entitlement: v2.Entitlement{
				DisplayName: "Session variables admin",
				Description: "Enable setting restricted session system variables",
				Annotations: nil,
				Purpose:     v2.Entitlement_PURPOSE_VALUE_PERMISSION,
			},
		},
		"set_user_id": {
			resourceTypes:    globalScope,
			v8Only:           true,
			includeWithGrant: true,
			entitlement: v2.Entitlement{
				DisplayName: "Set user ID",
				Description: "Enable setting non-self DEFINER values",
				Annotations: nil,
				Purpose:     v2.Entitlement_PURPOSE_VALUE_PERMISSION,
			},
		},
		"show_routine": {
			resourceTypes:    globalScope,
			v8Only:           true,
			includeWithGrant: true,
			entitlement: v2.Entitlement{
				DisplayName: "Show routine",
				Description: "Enable access to stored routine definitions",
				Annotations: nil,
				Purpose:     v2.Entitlement_PURPOSE_VALUE_PERMISSION,
			},
		},
		"skip_query_rewrite": {
			resourceTypes:    globalScope,
			v8Only:           true,
			includeWithGrant: true,
			entitlement: v2.Entitlement{
				DisplayName: "Skip query rewrite",
				Description: "Do not rewrite queries executed by this user",
				Annotations: nil,
				Purpose:     v2.Entitlement_PURPOSE_VALUE_PERMISSION,
			},
		},
		"system_user": {
			resourceTypes:    globalScope,
			v8Only:           true,
			includeWithGrant: true,
			entitlement: v2.Entitlement{
				DisplayName: "System user",
				Description: "Designate account as system account",
				Annotations: nil,
				Purpose:     v2.Entitlement_PURPOSE_VALUE_PERMISSION,
			},
		},
		"system_variables_admin": {
			resourceTypes:    globalScope,
			v8Only:           true,
			includeWithGrant: true,
			entitlement: v2.Entitlement{
				DisplayName: "System variables admin",
				Description: "Enable modifying or persisting global system variables",
				Annotations: nil,
				Purpose:     v2.Entitlement_PURPOSE_VALUE_PERMISSION,
			},
		},
		"table_encryption_admin": {
			resourceTypes:    globalScope,
			v8Only:           true,
			includeWithGrant: true,
			entitlement: v2.Entitlement{
				DisplayName: "Table encryption admin",
				Description: "Enable overriding default encryption settings",
				Annotations: nil,
				Purpose:     v2.Entitlement_PURPOSE_VALUE_PERMISSION,
			},
		},
		"version_token_admin": {
			resourceTypes:    globalScope,
			v8Only:           true,
			includeWithGrant: true,
			entitlement: v2.Entitlement{
				DisplayName: "Version token admin",
				Description: "Enable use of Version Tokens functions",
				Annotations: nil,
				Purpose:     v2.Entitlement_PURPOSE_VALUE_PERMISSION,
			},
		},
		"xa_recover_admin": {
			resourceTypes:    globalScope,
			v8Only:           true,
			includeWithGrant: true,
			entitlement: v2.Entitlement{
				DisplayName: "XA recover admin",
				Description: "Enable XA RECOVER execution",
				Annotations: nil,
				Purpose:     v2.Entitlement_PURPOSE_VALUE_PERMISSION,
			},
		},
		"role_assignment": {
			v8Only:           true,
			resourceTypes:    []*v2.ResourceType{resourceTypeUser, resourceTypeRole},
			includeWithGrant: true,
			entitlement: v2.Entitlement{
				DisplayName: "Role Member",
				Description: "Enables SET ROLE",
				Annotations: nil,
				Purpose:     v2.Entitlement_PURPOSE_VALUE_ASSIGNMENT,
			},
		},
	}

	for ID, et := range allEntitlements {
		for _, rt := range et.resourceTypes {
			newEt := et
			newEt.ID = ID

			entitlementsByResourceType[rt.Id] = append(entitlementsByResourceType[rt.Id], newEt)

			if et.includeWithGrant {
				grantEt := &entitlementTemplate{
					v8Only: et.v8Only,
					ID:     newEt.ID + "_with_grant",
					entitlement: v2.Entitlement{
						DisplayName: newEt.entitlement.DisplayName,
						Description: newEt.entitlement.Description,
						GrantableTo: newEt.entitlement.GrantableTo,
						Annotations: newEt.entitlement.Annotations,
						Purpose:     newEt.entitlement.Purpose,
					},
				}
				grantEt.entitlement.DisplayName = "Grant " + grantEt.entitlement.DisplayName
				entitlementsByResourceType[rt.Id] = append(entitlementsByResourceType[rt.Id], grantEt)
			}
		}
	}
}
