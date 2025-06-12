package client

import (
	"context"
	"fmt"
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

// If the privilege is "grant", it grants SELECT, INSERT, UPDATE, and REFERENCES privileges.
func (c *Client) GrantColumnPrivilege(ctx context.Context, table string, column string, user string, privilege string) error {
	userSplit := strings.Split(user, "@")
	if len(userSplit) != 2 {
		return fmt.Errorf("invalid user format: %s", user)
	}
	userGrant := fmt.Sprintf("%s'@'%s", userSplit[0], userSplit[1])

	var privileges []string
	if strings.ToLower(privilege) == "grant" {
		privileges = []string{"SELECT", "INSERT", "UPDATE", "REFERENCES"}
	} else {
		privileges = []string{strings.ToUpper(privilege)}
	}

	escapedTable, err := escapeMySQLIdent(table)
	if err != nil {
		return err
	}
	escapedColumn, err := escapeMySQLIdent(column)
	if err != nil {
		return err
	}

	var privilegeClauses []string
	for _, priv := range privileges {
		privilegeClauses = append(privilegeClauses, fmt.Sprintf("%s (%s)", priv, escapedColumn))
	}
	privilegesSQL := strings.Join(privilegeClauses, ", ")

	query := fmt.Sprintf("GRANT %s ON %s TO '%s'", privilegesSQL, escapedTable, userGrant)

	_ = c.db.MustExec(query)
	return nil
}

func (c *Client) RevokeColumnPrivilege(ctx context.Context, table string, column string, user string, privilege string) error {
	userSplit := strings.Split(user, "@")
	if len(userSplit) != 2 {
		return fmt.Errorf("invalid user format: %s", user)
	}
	userRevoke := fmt.Sprintf("%s'@'%s", userSplit[0], userSplit[1])

	var privileges []string
	if strings.ToLower(privilege) == "grant" {
		privileges = []string{"SELECT", "INSERT", "UPDATE", "REFERENCES"}
	} else {
		privileges = []string{strings.ToUpper(privilege)}
	}

	escapedTable, err := escapeMySQLIdent(table)
	if err != nil {
		return err
	}
	escapedColumn, err := escapeMySQLIdent(column)
	if err != nil {
		return err
	}

	var privilegeClauses []string
	for _, priv := range privileges {
		privilegeClauses = append(privilegeClauses, fmt.Sprintf("%s (%s)", priv, escapedColumn))
	}
	privilegesSQL := strings.Join(privilegeClauses, ", ")

	query := fmt.Sprintf("REVOKE %s ON %s FROM '%s'", privilegesSQL, escapedTable, userRevoke)

	_ = c.db.MustExec(query)
	return nil
}
