package client

import (
	"context"
	"strconv"
	"strings"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
)

const ColumnType = "column"

type ColumnModel struct {
	ID       string `db:"-"`
	Name     string `db:"COLUMN_NAME"`
	Database string `db:"TABLE_SCHEMA"`
	Table    string `db:"TABLE_NAME"`
}

// ListColumns scans the server for all columns associated with the parent table.
func (c *Client) ListColumns(ctx context.Context, parentResourceID *v2.ResourceId, pager *Pager) ([]*ColumnModel, string, error) {
	l := ctxzap.Extract(ctx)
	l.Debug("listing columns")

	parent, err := newDbResourceID(parentResourceID.Resource)
	if err != nil {
		return nil, "", err
	}

	offset, limit, err := pager.Parse()
	if err != nil {
		return nil, "", err
	}
	args := []interface{}{parent.DatabaseName, parent.ResourceName, limit + 1}

	var sb strings.Builder
	_, err = sb.WriteString("SELECT TABLE_NAME, TABLE_SCHEMA, COLUMN_NAME FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=? AND TABLE_NAME=? LIMIT ?")
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

	var ret []*ColumnModel
	for rows.Next() {
		var columnModel ColumnModel
		err = rows.StructScan(&columnModel)
		if err != nil {
			return nil, "", err
		}
		columnModel.ID = dbResourceID{
			ResourceTypeID:  ColumnType,
			DatabaseName:    parent.DatabaseName,
			ResourceName:    parent.ResourceName,
			SubResourceName: columnModel.Name,
		}.String()
		ret = append(ret, &columnModel)
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
