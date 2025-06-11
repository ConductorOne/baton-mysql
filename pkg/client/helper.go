package client

import (
	"fmt"
	"regexp"
	"strings"
)

// Helper for identifiers (tables, columns, databases)
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

// Helper for user/host
var validUserHost = regexp.MustCompile(`^[a-zA-Z0-9_%\\.\\-]+$`)

func escapeMySQLUserHost(ident string) (string, error) {
	if !validUserHost.MatchString(ident) {
		return "", fmt.Errorf("invalid user/host: %s", ident)
	}
	return ident, nil
}
