package client

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSplitUserHost(t *testing.T) {
	tests := []struct {
		name     string
		account  string
		wantUser string
		wantHost string
		wantErr  bool
	}{
		{name: "simple", account: "alice@localhost", wantUser: "alice", wantHost: "localhost"},
		{name: "email username", account: "jane.doe@example.com@%", wantUser: "jane.doe@example.com", wantHost: "%"},
		{name: "multiple at signs in username", account: "a@b@c@10.0.0.%", wantUser: "a@b@c", wantHost: "10.0.0.%"},
		{name: "anonymous account", account: "@localhost", wantUser: "", wantHost: "localhost"},
		{name: "collapsed hosts", account: "jane.doe@example.com@localhost,%", wantUser: "jane.doe@example.com", wantHost: "localhost,%"},
		{name: "netmask host", account: "bob@198.51.100.0/255.255.255.0", wantUser: "bob", wantHost: "198.51.100.0/255.255.255.0"},
		{name: "missing at sign", account: "alice", wantErr: true},
		{name: "empty host", account: "alice@", wantErr: true},
		{name: "empty", account: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, host, err := SplitUserHost(tt.account)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.wantUser, user)
			require.Equal(t, tt.wantHost, host)
		})
	}
}

func TestQuoteAccount(t *testing.T) {
	tests := []struct {
		name    string
		account string
		want    string
		wantErr bool
	}{
		{name: "simple", account: "alice@localhost", want: "'alice'@'localhost'"},
		{name: "email username", account: "jane.doe@example.com@%", want: "'jane.doe@example.com'@'%'"},
		{name: "host with hyphen", account: "svc-app@db-01.example.com", want: "'svc-app'@'db-01.example.com'"},
		{name: "quote injection in user", account: "bob' OR '1'='1@%", wantErr: true},
		{name: "quote injection in host", account: "bob@%' OR '1'='1", wantErr: true},
		{name: "trailing backslash escapes closing quote", account: `bob\@%`, wantErr: true},
		{name: "space in user", account: "bob smith@%", wantErr: true},
		{name: "anonymous account is not provisionable", account: "@localhost", wantErr: true},
		{name: "malformed", account: "alice", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := quoteAccount(tt.account)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}
