package client

import (
	"context"
	"fmt"
	"strings"
)

func (c *Client) GrantRolePrivilege(ctx context.Context, role, user, privilege string) error {
	roleParts := strings.Split(role, "@")
	if len(roleParts) != 2 {
		return fmt.Errorf("invalid role format: %s", role)
	}

	userParts := strings.Split(user, "@")
	if len(userParts) != 2 {
		return fmt.Errorf("invalid user format: %s", user)
	}

	roleUser, err := escapeMySQLUserHost(roleParts[0])
	if err != nil {
		return err
	}
	roleHost, err := escapeMySQLUserHost(roleParts[1])
	if err != nil {
		return err
	}
	targetUser, err := escapeMySQLUserHost(userParts[0])
	if err != nil {
		return err
	}
	targetHost, err := escapeMySQLUserHost(userParts[1])
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
	roleParts := strings.Split(role, "@")
	if len(roleParts) != 2 {
		return fmt.Errorf("invalid role format: %s", role)
	}

	userParts := strings.Split(user, "@")
	if len(userParts) != 2 {
		return fmt.Errorf("invalid user format: %s", user)
	}

	roleUser, err := escapeMySQLUserHost(roleParts[0])
	if err != nil {
		return err
	}
	roleHost, err := escapeMySQLUserHost(roleParts[1])
	if err != nil {
		return err
	}
	targetUser, err := escapeMySQLUserHost(userParts[0])
	if err != nil {
		return err
	}
	targetHost, err := escapeMySQLUserHost(userParts[1])
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
