package client

import (
	"testing"
)

func Test_SplitUserHost(t *testing.T) {
	type want struct {
		user string
		host string
	}
	tests := []struct {
		name    string
		in      string
		want    want
		wantErr bool
	}{
		{
			name: "simple user and host",
			in:   "someone@%",
			want: want{user: "someone", host: "%"},
		},
		{
			name: "username containing @",
			in:   "someone@orion.com@%",
			want: want{user: "someone@orion.com", host: "%"},
		},
		{
			name: "username containing multiple @",
			in:   "a@b@c@10.0.0.1",
			want: want{user: "a@b@c", host: "10.0.0.1"},
		},
		{
			name: "collapsed comma-separated hosts",
			in:   "someone@orion.com@localhost,%",
			want: want{user: "someone@orion.com", host: "localhost,%"},
		},
		{
			name:    "no @",
			in:      "someone",
			wantErr: true,
		},
		{
			name: "empty user (MySQL anonymous account)",
			in:   "@%",
			want: want{user: "", host: "%"},
		},
		{
			name:    "empty host",
			in:      "someone@",
			wantErr: true,
		},
		{
			name:    "empty string",
			in:      "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, host, err := SplitUserHost(tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("SplitUserHost(%q) error = %v, wantErr %v", tt.in, err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if user != tt.want.user || host != tt.want.host {
				t.Errorf("SplitUserHost(%q) = (%q, %q), want (%q, %q)", tt.in, user, host, tt.want.user, tt.want.host)
			}
		})
	}
}

func Test_escapeMySQLUserHost(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		wantErr bool
	}{
		{name: "empty (MySQL anonymous account username)", in: ""},
		{name: "plain username", in: "someone"},
		{name: "username with @", in: "someone@orion.com"},
		{name: "wildcard host", in: "%"},
		{name: "hostname", in: "%.example.com"},
		{name: "quote injection attempt", in: "someone' OR '1'='1", wantErr: true},
		{name: "space", in: "some one", wantErr: true},
		{name: "trailing backslash", in: `someone\`, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := escapeMySQLUserHost(tt.in)
			if (err != nil) != tt.wantErr {
				t.Errorf("escapeMySQLUserHost(%q) error = %v, wantErr %v", tt.in, err, tt.wantErr)
			}
		})
	}
}
