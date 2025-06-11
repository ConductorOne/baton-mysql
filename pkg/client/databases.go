package client

import (
	"context"
	"fmt"
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
	l.Debug("listing databases")

	offset, limit, err := pager.Parse()
	if err != nil {
		return nil, "", err
	}
	args := []interface{}{limit + 1}

	var sb strings.Builder
	_, err = sb.WriteString("SELECT SCHEMA_NAME FROM information_schema.SCHEMATA LIMIT ?")
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

func (c *Client) GrantDatabasePrivilege(ctx context.Context, database string, user string, privilege string) error {
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

	escapedDB, err := escapeMySQLIdent(database)
	if err != nil {
		return err
	}

	query := fmt.Sprintf("GRANT %s ON %s.* TO %s", strings.ToUpper(privilege), escapedDB, userGrant)
	_ = c.db.MustExec(query)
	return nil
}

func (c *Client) RevokeDatabasePrivilege(ctx context.Context, database string, user string, privilege string) error {
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

	escapedDB, err := escapeMySQLIdent(database)
	if err != nil {
		return err
	}

	query := fmt.Sprintf("REVOKE %s ON %s.* FROM %s", strings.ToUpper(privilege), escapedDB, userRevoke)
	_ = c.db.MustExec(query)
	return nil
}
