package client

import (
	"database/sql"
	"testing"
)

// The grant and entitlement ids are built from roleID, so it has to keep
// producing what User.GetID produces for the same account. If the two drift,
// every role grant is re-keyed and C1 replaces the old ones silently.
func TestRoleIDMatchesUserGetID(t *testing.T) {
	for _, tc := range []struct {
		name     string
		roleType string
		user     string
		host     string
	}{
		{"role", RoleType, "ops@team", "%"},
		{"user granted as a role", UserType, "dup", "localhost"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			edge := &RoleGrant{
				RoleType: sql.NullString{String: tc.roleType, Valid: true},
				FromUser: tc.user,
				FromHost: tc.host,
			}
			account := &User{UserType: tc.roleType, User: tc.user, Host: tc.host}

			if got, want := edge.roleID(false), account.GetID(); got != want {
				t.Errorf("roleID() = %q, User.GetID() = %q", got, want)
			}
		})
	}
}

// Under collapseUsers a user-typed account is one identity across its hosts, so
// the id carries the host list ListUsers groups it under. Roles are never
// collapsed.
func TestRoleIDUnderCollapse(t *testing.T) {
	for _, tc := range []struct {
		name      string
		roleType  string
		fromHosts sql.NullString
		want      string
	}{
		{"user keeps the grouped hosts", UserType, sql.NullString{String: "%,localhost", Valid: true}, "user:dup@%,localhost"},
		{"role is not collapsed", RoleType, sql.NullString{String: "%,localhost", Valid: true}, "role:dup@%"},
		{"user with no grouped row falls back", UserType, sql.NullString{}, "user:dup@%"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			edge := &RoleGrant{
				RoleType:  sql.NullString{String: tc.roleType, Valid: true},
				FromUser:  "dup",
				FromHost:  "%",
				FromHosts: tc.fromHosts,
			}
			if got := edge.roleID(true); got != tc.want {
				t.Errorf("roleID(collapse) = %q, want %q", got, tc.want)
			}
		})
	}
}

// Two accounts can share a user name and differ only in host, and that
// membership is real — except under collapseUsers, where those hosts are one
// identity holding itself.
func TestHeldBySameAccount(t *testing.T) {
	for _, tc := range []struct {
		name     string
		roleType string
		fromHost string
		toHost   string
		collapse bool
		want     bool
	}{
		{"same account", UserType, "%", "%", false, true},
		{"same name, different host, is a real membership", UserType, "%", "localhost", false, false},
		{"same name, different host, is one identity under collapse", UserType, "%", "localhost", true, true},
		{"a role is not collapsed", RoleType, "%", "localhost", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			edge := &RoleGrant{
				RoleType: sql.NullString{String: tc.roleType, Valid: true},
				FromUser: "dup", FromHost: tc.fromHost,
				ToUser: "dup", ToHost: tc.toHost,
			}
			if got := edge.heldBySameAccount(tc.collapse); got != tc.want {
				t.Errorf("heldBySameAccount(%v) = %v, want %v", tc.collapse, got, tc.want)
			}
		})
	}
}
