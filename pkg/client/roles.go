package client

import (
	"context"
	"fmt"
)

func (c *Client) GrantRolePrivilege(ctx context.Context, role, user, privilege string) error {
	roleAccount, err := quoteAccount(role)
	if err != nil {
		return fmt.Errorf("invalid role %s: %w", role, err)
	}
	userAccount, err := quoteAccount(user)
	if err != nil {
		return fmt.Errorf("invalid user %s: %w", user, err)
	}

	var grantStmt string
	switch privilege {
	case "role_assignment":
		grantStmt = fmt.Sprintf("GRANT %s TO %s", roleAccount, userAccount)
	case "role_assignment_with_grant":
		grantStmt = fmt.Sprintf("GRANT %s TO %s WITH ADMIN OPTION", roleAccount, userAccount)
	case "proxy":
		grantStmt = fmt.Sprintf("GRANT PROXY ON %s TO %s", roleAccount, userAccount)
	case "proxy_with_grant":
		grantStmt = fmt.Sprintf("GRANT PROXY ON %s TO %s WITH GRANT OPTION", roleAccount, userAccount)
	default:
		return fmt.Errorf("unknown privilege: %s", privilege)
	}

	_ = c.db.MustExec(grantStmt)
	return nil
}

func (c *Client) RevokeRolePrivilege(ctx context.Context, role, user, privilege string) error {
	roleAccount, err := quoteAccount(role)
	if err != nil {
		return fmt.Errorf("invalid role %s: %w", role, err)
	}
	userAccount, err := quoteAccount(user)
	if err != nil {
		return fmt.Errorf("invalid user %s: %w", user, err)
	}

	var revokeStmt string
	switch privilege {
	case "role_assignment", "role_assignment_with_grant":
		revokeStmt = fmt.Sprintf("REVOKE %s FROM %s", roleAccount, userAccount)
	case "proxy", "proxy_with_grant":
		revokeStmt = fmt.Sprintf("REVOKE PROXY ON %s FROM %s", roleAccount, userAccount)
	default:
		return fmt.Errorf("unknown privilege: %s", privilege)
	}

	_ = c.db.MustExec(revokeStmt)
	return nil
}
