package client

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
)

const ServerType = "server"

type ServerModel struct {
	ID      string `db:"-"`
	Name    string `db:"hostname"`
	Version string `db:"version"`
}

func (c *Client) GetServerInfo(ctx context.Context) (*ServerModel, error) {
	l := ctxzap.Extract(ctx)
	l.Debug("getting server")

	s := ServerModel{}
	err := c.db.GetContext(ctx, &s, "SELECT @@hostname hostname, @@version version")
	if err != nil {
		return nil, err
	}

	s.ID = fmt.Sprintf("%s:%s", ServerType, s.Name)

	return &s, nil
}

func (c *Client) ExecContext(ctx context.Context, query string) (sql.Result, error) {
	if c.db == nil {
		return nil, fmt.Errorf("database connection is not initialized")
	}
	return c.db.ExecContext(ctx, query)
}

func (c *Client) GrantServerPrivilege(ctx context.Context, user string, privilege string) error {
	userSplit := strings.Split(user, "@")
	if len(userSplit) != 2 {
		return fmt.Errorf("invalid user format, expected user@host")
	}
	userEsc, err := escapeMySQLUserHost(userSplit[0])
	if err != nil {
		return err
	}
	hostEsc, err := escapeMySQLUserHost(userSplit[1])
	if err != nil {
		return err
	}
	userGrant := fmt.Sprintf("'%s'@'%s'", userEsc, hostEsc)

	query := fmt.Sprintf("GRANT %s ON *.* TO %s", strings.ToUpper(privilege), userGrant)
	_ = c.db.MustExec(query)
	return nil
}

func (c *Client) RevokeServerPrivilege(ctx context.Context, user string, privilege string) error {
	userSplit := strings.Split(user, "@")
	if len(userSplit) != 2 {
		return fmt.Errorf("invalid user format, expected user@host")
	}
	userEsc, err := escapeMySQLUserHost(userSplit[0])
	if err != nil {
		return err
	}
	hostEsc, err := escapeMySQLUserHost(userSplit[1])
	if err != nil {
		return err
	}
	userRevoke := fmt.Sprintf("'%s'@'%s'", userEsc, hostEsc)

	query := fmt.Sprintf("REVOKE %s ON *.* FROM %s", strings.ToUpper(privilege), userRevoke)
	_ = c.db.MustExec(query)
	return nil
}
