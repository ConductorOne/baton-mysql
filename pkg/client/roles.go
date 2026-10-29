package client

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/go-sql-driver/mysql"
)

const (
	privRoleAssignment          = "role_assignment"
	privRoleAssignmentWithGrant = "role_assignment_with_grant"
)

// splitCollapsedAccount validates a user@host account name and returns one quoted
// 'user'@'host' account per host variant. Collapsed users (user@host1,host2) expand to
// one account per host; plain accounts expand to a single-element slice.
func splitCollapsedAccount(account string) ([]string, error) {
	user, hosts, err := SplitUserHost(account)
	if err != nil {
		return nil, err
	}
	var quoted []string
	for _, host := range strings.Split(hosts, ",") {
		q, err := quoteUserHost(user, host)
		if err != nil {
			return nil, err
		}
		quoted = append(quoted, q)
	}
	return quoted, nil
}

// isNoSuchGrantErr reports whether err is MySQL ER_NONEXISTING_GRANT (1147), which
// REVOKE raises when the named account does not hold the privilege. Tolerating it
// makes revokes idempotent: revoking an entitlement already absent is success.
func isNoSuchGrantErr(err error) bool {
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) {
		return mysqlErr.Number == 1147
	}
	return false
}

func (c *Client) GrantRolePrivilege(ctx context.Context, role, user, privilege string) error {
	roleAccount, err := quoteAccount(role)
	if err != nil {
		return fmt.Errorf("invalid role %s: %w", role, err)
	}
	userAccount, err := quoteAccounts(user)
	if err != nil {
		return fmt.Errorf("invalid user %s: %w", user, err)
	}

	var grantStmt string
	switch privilege {
	case privRoleAssignment:
		grantStmt = fmt.Sprintf("GRANT %s TO %s", roleAccount, userAccount)
	case privRoleAssignmentWithGrant:
		grantStmt = fmt.Sprintf("GRANT %s TO %s WITH ADMIN OPTION", roleAccount, userAccount)
	case "proxy":
		grantStmt = fmt.Sprintf("GRANT PROXY ON %s TO %s", roleAccount, userAccount)
	case "proxy_with_grant":
		grantStmt = fmt.Sprintf("GRANT PROXY ON %s TO %s WITH GRANT OPTION", roleAccount, userAccount)
	default:
		return fmt.Errorf("unknown privilege: %s", privilege)
	}

	_, err = c.db.ExecContext(ctx, grantStmt)
	return err
}

func (c *Client) RevokeRolePrivilege(ctx context.Context, role, user, privilege string) error {
	roleAccount, err := quoteAccount(role)
	if err != nil {
		return fmt.Errorf("invalid role %s: %w", role, err)
	}
	userAccount, err := quoteAccounts(user)
	if err != nil {
		return fmt.Errorf("invalid user %s: %w", user, err)
	}

	if isCollapsedAccount(user) && (privilege == privRoleAssignment || privilege == privRoleAssignmentWithGrant) {
		return c.revokeRolePerVariant(ctx, roleAccount, user)
	}

	var revokeStmt string
	switch privilege {
	case privRoleAssignment, privRoleAssignmentWithGrant:
		revokeStmt = fmt.Sprintf("REVOKE %s FROM %s", roleAccount, userAccount)
	case "proxy", "proxy_with_grant":
		revokeStmt = fmt.Sprintf("REVOKE PROXY ON %s FROM %s", roleAccount, userAccount)
	default:
		return fmt.Errorf("unknown privilege: %s", privilege)
	}

	_, err = c.db.ExecContext(ctx, revokeStmt)
	return err
}

// isCollapsedAccount reports whether the account's host part lists multiple hosts.
func isCollapsedAccount(account string) bool {
	_, hosts, err := SplitUserHost(account)
	if err != nil {
		return false
	}
	return strings.Contains(hosts, ",")
}

// revokeRolePerVariant revokes the role from each host variant of a collapsed
// principal separately. MySQL rejects a multi-account REVOKE outright when any named
// account does not hold the grant (ER_NONEXISTING_GRANT, 1147), so a membership edge
// held by only some host variants would fail the whole statement. Revoking per
// variant and tolerating 1147 per variant keeps the revoke idempotent: state already
// matching the desired state is success. Every other error fails wrapped.
func (c *Client) revokeRolePerVariant(ctx context.Context, roleAccount, user string) error {
	variants, err := splitCollapsedAccount(user)
	if err != nil {
		return fmt.Errorf("invalid user %s: %w", user, err)
	}
	for _, variant := range variants {
		stmt := fmt.Sprintf("REVOKE %s FROM %s", roleAccount, variant)
		if _, err := c.db.ExecContext(ctx, stmt); err != nil && !isNoSuchGrantErr(err) {
			return fmt.Errorf("revoke %s from %s: %w", roleAccount, variant, err)
		}
	}
	return nil
}
