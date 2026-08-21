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

// Helper for user/host. Empty is allowed: every caller derives ident via
// SplitUserHost, which guarantees the host half is always non-empty, so an
// empty string here can only be the username of MySQL's anonymous account
// (''@'host'). ":" and "/" are allowed because MySQL host specs include IPv6
// literals (the stock root@::1) and netmask forms (198.51.100.0/255.255.255.0);
// both are inert inside the single-quoted '%s'@'%s' the callers build. "'" and
// "\" stay excluded, as those are what could break out of that quoting.
var validUserHost = regexp.MustCompile(`^[a-zA-Z0-9_%.@:/\-]*$`)

func escapeMySQLUserHost(ident string) (string, error) {
	if !validUserHost.MatchString(ident) {
		return "", fmt.Errorf("invalid user/host: %s", ident)
	}
	return ident, nil
}

// SplitUserHost splits a "name@host" identifier into its name and host parts.
// Names (MySQL usernames or role names) may themselves legally contain "@",
// but MySQL host specifications (hostnames, IPs, netmasks, or "%" wildcards)
// never do, so splitting on the last "@" unambiguously recovers both parts.
func SplitUserHost(s string) (string, string, error) {
	idx := strings.LastIndex(s, "@")
	// An empty name is valid: MySQL's anonymous account is ''@'host'.
	if idx < 0 || idx == len(s)-1 {
		return "", "", fmt.Errorf("invalid user@host format: %s", s)
	}
	return s[:idx], s[idx+1:], nil
}
