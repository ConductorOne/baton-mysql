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

	roleUser := roleParts[0]
	roleHost := roleParts[1]

	targetUser := userParts[0]
	targetHost := userParts[1]

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

	_, err := c.db.ExecContext(ctx, grantStmt)
	return err
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

	roleUser := roleParts[0]
	roleHost := roleParts[1]

	targetUser := userParts[0]
	targetHost := userParts[1]

	var revokeStmt string
	switch privilege {
	case "role_assignment", "role_assignment_with_grant":
		revokeStmt = fmt.Sprintf("REVOKE '%s'@'%s' FROM '%s'@'%s'", roleUser, roleHost, targetUser, targetHost)
	case "proxy", "proxy_with_grant":
		revokeStmt = fmt.Sprintf("REVOKE PROXY ON '%s'@'%s' FROM '%s'@'%s'", roleUser, roleHost, targetUser, targetHost)
	default:
		return fmt.Errorf("unknown privilege: %s", privilege)
	}

	_, err := c.db.ExecContext(ctx, revokeStmt)
	return err
}
