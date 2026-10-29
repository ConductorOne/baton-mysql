package client

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/go-sql-driver/mysql"
	sqlx "github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"
)

func TestSplitCollapsedAccount(t *testing.T) {
	tests := []struct {
		name    string
		account string
		want    []string
		wantErr bool
	}{
		{name: "single host", account: "alice@localhost", want: []string{"'alice'@'localhost'"}},
		{name: "collapsed hosts", account: "jane.doe@example.com@localhost,%", want: []string{"'jane.doe@example.com'@'localhost'", "'jane.doe@example.com'@'%'"}},
		{name: "three variants", account: "bob@h1,h2,h3", want: []string{"'bob'@'h1'", "'bob'@'h2'", "'bob'@'h3'"}},
		{name: "anonymous account", account: "@localhost", wantErr: true},
		{name: "missing at sign", account: "alice", wantErr: true},
		{name: "empty host", account: "alice@", wantErr: true},
		{name: "empty host variant", account: "alice@h1,,h2", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := splitCollapsedAccount(tt.account)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestIsCollapsedAccount(t *testing.T) {
	require.True(t, isCollapsedAccount("alice@h1,h2"))
	require.False(t, isCollapsedAccount("alice@localhost"))
	require.False(t, isCollapsedAccount("alice"))
}

func TestIsNoSuchGrantErr(t *testing.T) {
	require.True(t, isNoSuchGrantErr(&mysql.MySQLError{Number: 1147, Message: "There is no such grant defined for user 'x' on host 'h'"}))
	require.False(t, isNoSuchGrantErr(&mysql.MySQLError{Number: 1142, Message: "DELETE command denied"}))
	require.False(t, isNoSuchGrantErr(errors.New("generic driver error")))
	require.False(t, isNoSuchGrantErr(nil))
}

func TestRevokeRolePrivilegePerVariant(t *testing.T) {
	// noSuchGrantOn returns a driver whose statements fail with ER_NONEXISTING_GRANT
	// (1147) when the executed statement contains the given account fragment, and
	// succeed otherwise. Other numbers fail every statement.
	noSuchGrantOn := func(failOn string, number uint16) *revokeRecorder {
		return &revokeRecorder{failOn: failOn, errNumber: number}
	}

	tests := []struct {
		name      string
		user      string
		privilege string
		rec       *revokeRecorder
		wantExec  []string
		wantErr   bool
	}{
		{
			name:      "collapsed user, edge held by only h1: per-variant revoke tolerates 1147 on h1",
			user:      "alice@h1,h2",
			privilege: "role_assignment",
			rec:       noSuchGrantOn("'alice'@'h1'", 1147),
			wantExec:  []string{"REVOKE 'r_role'@'%' FROM 'alice'@'h1'", "REVOKE 'r_role'@'%' FROM 'alice'@'h2'"},
		},
		{
			name:      "collapsed user with admin option: one revoke per variant",
			user:      "alice@h1,h2",
			privilege: "role_assignment_with_grant",
			rec:       noSuchGrantOn("", 1147),
			wantExec:  []string{"REVOKE 'r_role'@'%' FROM 'alice'@'h1'", "REVOKE 'r_role'@'%' FROM 'alice'@'h2'"},
		},
		{
			name:      "non-collapsed user keeps single-statement behavior",
			user:      "alice@h1",
			privilege: "role_assignment",
			rec:       noSuchGrantOn("", 0),
			wantExec:  []string{"REVOKE 'r_role'@'%' FROM 'alice'@'h1'"},
		},
		{
			name:      "non-collapsed user, 1147 still errors (no behavior change)",
			user:      "alice@h1",
			privilege: "role_assignment",
			rec:       noSuchGrantOn("", 1147),
			wantErr:   true,
		},
		{
			name:      "collapsed user, non-1147 error fails wrapped",
			user:      "alice@h1,h2",
			privilege: "role_assignment",
			rec:       noSuchGrantOn("", 1142),
			wantErr:   true,
		},
		{
			name:      "collapsed proxy revoke keeps single-statement behavior",
			user:      "alice@h1,h2",
			privilege: "proxy",
			rec:       noSuchGrantOn("", 0),
			wantExec:  []string{"REVOKE PROXY ON 'r_role'@'%' FROM 'alice'@'h1', 'alice'@'h2'"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Client{db: sqlx.NewDb(sql.OpenDB(tt.rec), "mysql")}
			err := c.RevokeRolePrivilege(context.Background(), "r_role@%", tt.user, tt.privilege)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.wantExec, tt.rec.executed())
		})
	}
}

func TestRevokeRolePrivilegeUnknownPrivilege(t *testing.T) {
	c := &Client{db: sqlx.NewDb(sql.OpenDB(&revokeRecorder{}), "mysql")}
	err := c.RevokeRolePrivilege(context.Background(), "r_role@%", "alice@h1", "bogus")
	require.ErrorContains(t, err, "unknown privilege")
}

// revokeRecorder is a database/sql driver connector that records every executed
// statement and optionally fails statements containing failOn with a MySQLError
// carrying errNumber (0 means never fail).
type revokeRecorder struct {
	mu        sync.Mutex
	failOn    string
	errNumber uint16
	exec      []string
}

func (r *revokeRecorder) executed() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.exec...)
}

func (r *revokeRecorder) Connect(context.Context) (driver.Conn, error) { return revokeConn{r}, nil }
func (r *revokeRecorder) Driver() driver.Driver                        { return revokeDriver{} }

type revokeDriver struct{}

func (revokeDriver) Open(string) (driver.Conn, error) { return nil, errors.New("not used") }

type revokeConn struct{ rec *revokeRecorder }

func (c revokeConn) Prepare(query string) (driver.Stmt, error) {
	return revokeStmt{rec: c.rec, query: query}, nil
}
func (revokeConn) Close() error              { return nil }
func (revokeConn) Begin() (driver.Tx, error) { return nil, errors.New("not used") }

type revokeStmt struct {
	rec   *revokeRecorder
	query string
}

func (revokeStmt) Close() error  { return nil }
func (revokeStmt) NumInput() int { return -1 }

func (s revokeStmt) Exec([]driver.Value) (driver.Result, error) {
	s.rec.mu.Lock()
	s.rec.exec = append(s.rec.exec, s.query)
	s.rec.mu.Unlock()
	if s.rec.errNumber != 0 && strings.Contains(s.query, s.rec.failOn) {
		return nil, &mysql.MySQLError{Number: s.rec.errNumber, Message: "test error"}
	}
	return driver.RowsAffected(1), nil
}

func (revokeStmt) Query([]driver.Value) (driver.Rows, error) { return nil, errors.New("not used") }
