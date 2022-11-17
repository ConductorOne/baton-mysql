package client

import (
	"context"
	"strconv"
	"strings"

	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
)

const DatabaseType = "database"

type DbModel struct {
	ID   string `db:"-"`
	Name string `db:"SCHEMA_NAME"`
}

// ListDatabases scans and returns all the databases.
func (c *Client) ListDatabases(ctx context.Context, pager *Pager) ([]*DbModel, string, error) {
	l := ctxzap.Extract(ctx)
	l.Info("listing databases")

	offset, limit, err := pager.Parse()
	if err != nil {
		return nil, "", err
	}
	args := []interface{}{limit + 1}

	var sb strings.Builder
	sb.WriteString("SELECT SCHEMA_NAME FROM information_schema.SCHEMATA LIMIT ?")
	if offset > 0 {
		sb.WriteString(" OFFSET ?")
		args = append(args, offset)
	}

	rows, err := c.db.QueryxContext(ctx, sb.String(), args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	var ret []*DbModel
	for rows.Next() {
		var dbModel DbModel
		err = rows.StructScan(&dbModel)
		if err != nil {
			return nil, "", err
		}
		dbModel.ID = dbResourceID{
			ResourceTypeID: DatabaseType,
			DatabaseName:   dbModel.Name,
		}.String()
		ret = append(ret, &dbModel)
	}
	if rows.Err() != nil {
		return nil, "", rows.Err()
	}

	var nextPageToken string
	if len(ret) > limit {
		offset += limit
		nextPageToken = strconv.Itoa(offset)
		ret = ret[:limit]
	}

	return ret, nextPageToken, nil
}
