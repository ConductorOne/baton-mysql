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
		{name: "space in user", account: "bob smith@%", want: "'bob smith'@'%'"},
		{name: "netmask host", account: "bob@198.51.100.0/255.255.255.0", want: "'bob'@'198.51.100.0/255.255.255.0'"},
		{name: "ipv6 host", account: "bob@::1", want: "'bob'@'::1'"},
		{name: "collapsed hosts are a single-account error", account: "bob@localhost,%", wantErr: true},
		{name: "unsupported character", account: "bob#1@%", wantErr: true},
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

func TestQuoteString(t *testing.T) {
	tests := []struct {
		name               string
		input              string
		noBackslashEscapes bool
		want               string
	}{
		{name: "plain", input: "Abc123!", want: "'Abc123!'"},
		{name: "single quote", input: "a'b", want: "'a''b'"},
		{name: "trailing backslash", input: `ab\`, want: `'ab\\'`},
		{name: "backslash sequence", input: `a\nb`, want: `'a\\nb'`},
		{name: "quote and backslash", input: `x'y\z`, want: `'x''y\\z'`},
		{name: "no backslash escapes keeps backslash", input: `ab\`, noBackslashEscapes: true, want: `'ab\'`},
		{name: "no backslash escapes still doubles quote", input: `x'y\z`, noBackslashEscapes: true, want: `'x''y\z'`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, quoteString(tt.input, tt.noBackslashEscapes))
		})
	}
}

func TestQuoteAccounts(t *testing.T) {
	tests := []struct {
		name    string
		account string
		want    string
		wantErr bool
	}{
		{name: "single host", account: "alice@localhost", want: "'alice'@'localhost'"},
		{name: "collapsed hosts", account: "jane.doe@example.com@localhost,%", want: "'jane.doe@example.com'@'localhost', 'jane.doe@example.com'@'%'"},
		{name: "collapsed ipv6 and netmask hosts", account: "bob@::1,10.0.0.0/255.0.0.0", want: "'bob'@'::1', 'bob'@'10.0.0.0/255.0.0.0'"},
		{name: "empty host in list", account: "bob@localhost,", wantErr: true},
		{name: "quote injection in one host", account: "bob@localhost,%' OR '1'='1", wantErr: true},
		{name: "anonymous account", account: "@localhost,%", wantErr: true},
		{name: "malformed", account: "alice", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := quoteAccounts(tt.account)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}
