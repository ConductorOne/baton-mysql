package client

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	grantPrivilege = "grant"
	proxyPrivilege = "proxy"
)

func privilegeKey(name string) string {
	return strings.NewReplacer(" ", "", "_", "").Replace(strings.ToLower(name))
}

func splitGrantSuffix(slug string) (string, bool) {
	const suffix = "_with_grant"
	lowered := strings.ToLower(slug)
	if strings.HasSuffix(lowered, suffix) {
		return strings.TrimSuffix(lowered, suffix), true
	}
	return lowered, false
}

// matchPrivilege returns the spelling SHOW PRIVILEGES uses for slug. withGrant
// reports the *_with_grant form and is returned on its own because the clause
// is not part of the name; the caller places it in the statement.
func matchPrivilege(slug string, registered []string) (string, bool, error) {
	base, withGrant := splitGrantSuffix(slug)
	if base == proxyPrivilege {
		return "", false, status.Error(codes.InvalidArgument, "proxy is granted on an account, not on *.*")
	}

	key := privilegeKey(base)
	if key == grantPrivilege {
		key = "grantoption"
	}
	for _, name := range registered {
		if privilegeKey(name) == key {
			return strings.ToUpper(name), withGrant, nil
		}
	}
	return "", false, status.Errorf(codes.InvalidArgument, "unrecognized privilege %q", slug)
}

func (c *Client) registeredPrivileges(ctx context.Context) ([]string, error) {
	rows, err := c.db.QueryxContext(ctx, "SHOW PRIVILEGES")
	if err != nil {
		return nil, fmt.Errorf("list privileges: %w", err)
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var privilege, scope, comment string
		if err := rows.Scan(&privilege, &scope, &comment); err != nil {
			return nil, fmt.Errorf("read privilege: %w", err)
		}
		names = append(names, privilege)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list privileges: %w", err)
	}
	return names, nil
}

func (c *Client) SQLPrivilege(ctx context.Context, slug string) (string, bool, error) {
	names, err := c.registeredPrivileges(ctx)
	if err != nil {
		return "", false, err
	}
	return matchPrivilege(slug, names)
}

func withGrantOption(query string, withGrant bool) string {
	if !withGrant {
		return query
	}
	return query + " WITH GRANT OPTION"
}
