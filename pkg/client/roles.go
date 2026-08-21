package client

import (
	"context"
	"fmt"
)

func (c *Client) GrantRolePrivilege(ctx context.Context, role, user, privilege string) error {
	roleName, roleHostRaw, err := SplitUserHost(role)
	if err != nil {
		return fmt.Errorf("invalid role format: %s: %w", role, err)
	}

	userName, userHostRaw, err := SplitUserHost(user)
	if err != nil {
		return fmt.Errorf("invalid user format: %s: %w", user, err)
	}

	roleUser, err := escapeMySQLUserHost(roleName)
	if err != nil {
		return err
	}
	roleHost, err := escapeMySQLUserHost(roleHostRaw)
	if err != nil {
		return err
	}
	targetUser, err := escapeMySQLUserHost(userName)
	if err != nil {
		return err
	}
	targetHost, err := escapeMySQLUserHost(userHostRaw)
	if err != nil {
		return err
	}

	var grantStmt string
	switch privilege {
	case "role_assignment":
		grantStmt = fmt.Sprintf("GRANT '%s'@'%s' TO '%s'@'%s'", roleUser, roleHost, targetUser, targetHost)
	case "role_assignment_with_grant":
		grantStmt = fmt.Sprintf("GRANT '%s'@'%s' TO '%s'@'%s' WITH ADMIN OPTION", roleUser, roleHost, targetUser, targetHost)
	case "proxy":
		grantStmt = fmt.Sprintf("GRANT PROXY ON '%s'@'%s' TO '%s'@'%s'", roleUser, roleHost, targetUser, targetHost)
	case "proxy_with_grant":
		grantStmt = fmt.Sprintf("GRANT PROXY ON '%s'@'%s' TO '%s'@'%s' WITH GRANT OPTION", roleUser, roleHost, targetUser, targetHost)
	default:
		return fmt.Errorf("unknown privilege: %s", privilege)
	}

	_ = c.db.MustExec(grantStmt)
	return nil
}

func (c *Client) RevokeRolePrivilege(ctx context.Context, role, user, privilege string) error {
	roleName, roleHostRaw, err := SplitUserHost(role)
	if err != nil {
		return fmt.Errorf("invalid role format: %s: %w", role, err)
	}

	userName, userHostRaw, err := SplitUserHost(user)
	if err != nil {
		return fmt.Errorf("invalid user format: %s: %w", user, err)
	}

	roleUser, err := escapeMySQLUserHost(roleName)
	if err != nil {
		return err
	}
	roleHost, err := escapeMySQLUserHost(roleHostRaw)
	if err != nil {
		return err
	}
	targetUser, err := escapeMySQLUserHost(userName)
	if err != nil {
		return err
	}
	targetHost, err := escapeMySQLUserHost(userHostRaw)
	if err != nil {
		return err
	}

	var revokeStmt string
	switch privilege {
	case "role_assignment", "role_assignment_with_grant":
		revokeStmt = fmt.Sprintf("REVOKE '%s'@'%s' FROM '%s'@'%s'", roleUser, roleHost, targetUser, targetHost)
	case "proxy", "proxy_with_grant":
		revokeStmt = fmt.Sprintf("REVOKE PROXY ON '%s'@'%s' FROM '%s'@'%s'", roleUser, roleHost, targetUser, targetHost)
	default:
		return fmt.Errorf("unknown privilege: %s", privilege)
	}

	_ = c.db.MustExec(revokeStmt)
	return nil
}
