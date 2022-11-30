package client

import (
	"context"
	"fmt"

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
