package client

import (
	"context"
	"strconv"
	"strings"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
)

const TableType = "table"

type TableModel struct {
	ID       string `db:"-"`
	Name     string `db:"TABLE_NAME"`
	Database string `db:"TABLE_SCHEMA"`
	Type     string `db:"TABLE_TYPE"`
}

// ListTables scans and returns all the tables for the parent database.
func (c *Client) ListTables(ctx context.Context, parentResourceID *v2.ResourceId, pager *Pager) ([]*TableModel, string, error) {
	l := ctxzap.Extract(ctx)
	l.Debug("listing Tables")

	parent, err := newDbResourceID(parentResourceID.Resource)
	if err != nil {
		return nil, "", err
	}

	offset, limit, err := pager.Parse()
	if err != nil {
		return nil, "", err
	}
	args := []interface{}{parent.DatabaseName, limit + 1}

	var sb strings.Builder
	_, err = sb.WriteString("SELECT TABLE_NAME, TABLE_SCHEMA, TABLE_TYPE FROM information_schema.TABLES WHERE TABLE_SCHEMA=? LIMIT ?")
	if err != nil {
		return nil, "", err
	}
	if offset > 0 {
		_, err = sb.WriteString(" OFFSET ?")
		if err != nil {
			return nil, "", err
		}
		args = append(args, offset)
	}

	rows, err := c.db.QueryxContext(ctx, sb.String(), args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	var ret []*TableModel
	for rows.Next() {
		var tableModel TableModel
		err = rows.StructScan(&tableModel)
		if err != nil {
			return nil, "", err
		}
		tableModel.ID = dbResourceID{
			ResourceTypeID: TableType,
			DatabaseName:   parent.DatabaseName,
			ResourceName:   tableModel.Name,
		}.String()
		ret = append(ret, &tableModel)
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
