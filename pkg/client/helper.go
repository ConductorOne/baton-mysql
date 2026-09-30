package client

import (
	"fmt"
	"regexp"
	"strings"
)

// Helper for identifiers (tables, columns, databases).
var validIdent = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)

func escapeMySQLIdent(ident string) (string, error) {
	parts := strings.Split(ident, ".")
	for i, part := range parts {
		if !validIdent.MatchString(part) {
			return "", fmt.Errorf("invalid identifier: %s", ident)
		}
		parts[i] = "`" + strings.ReplaceAll(part, "`", "``") + "`"
	}
	return strings.Join(parts, "."), nil
}

// Helper for user/host. Backslash is excluded because it would escape the closing quote.
var validUserHost = regexp.MustCompile(`^[a-zA-Z0-9_%.@\-]+$`)

func escapeMySQLUserHost(ident string) (string, error) {
	if !validUserHost.MatchString(ident) {
		return "", fmt.Errorf("invalid user/host: %s", ident)
	}
	return ident, nil
}

// SplitUserHost splits a user@host account name on the last '@': MySQL allows '@' in user names but rejects it in host names.
// An empty user is valid, since MySQL's anonymous account has a blank user name.
func SplitUserHost(account string) (string, string, error) {
	idx := strings.LastIndex(account, "@")
	if idx < 0 || idx == len(account)-1 {
		return "", "", fmt.Errorf("invalid account %q, expected user@host", account)
	}
	return account[:idx], account[idx+1:], nil
}

// quoteAccount validates a user@host account name and returns it quoted as 'user'@'host'.
func quoteAccount(account string) (string, error) {
	user, host, err := SplitUserHost(account)
	if err != nil {
		return "", err
	}
	userEsc, err := escapeMySQLUserHost(user)
	if err != nil {
		return "", err
	}
	hostEsc, err := escapeMySQLUserHost(host)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("'%s'@'%s'", userEsc, hostEsc), nil
}
