package client

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
)

const RoutineType = "routine"

type RoutineModel struct {
	ID       string `db:"-"`
	Name     string `db:"SPECIFIC_NAME"`
	Database string `db:"ROUTINE_SCHEMA"`
	Type     string `db:"ROUTINE_TYPE"`
}

// ListRoutines scans and returns all the routines associated with the parent database.
func (c *Client) ListRoutines(ctx context.Context, parentResourceID *v2.ResourceId, pager *Pager) ([]*RoutineModel, string, error) {
	l := ctxzap.Extract(ctx)
	l.Debug("listing routines")

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
	_, err = sb.WriteString("SELECT SPECIFIC_NAME, ROUTINE_SCHEMA, ROUTINE_TYPE FROM information_schema.ROUTINES WHERE ROUTINE_SCHEMA=? LIMIT ?")
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

	var ret []*RoutineModel
	for rows.Next() {
		var routineModel RoutineModel
		err = rows.StructScan(&routineModel)
		if err != nil {
			return nil, "", err
		}

		routineModel.ID = dbResourceID{
			ResourceTypeID: RoutineType,
			DatabaseName:   parent.DatabaseName,
			ResourceName:   routineModel.Name,
		}.String()
		ret = append(ret, &routineModel)
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

func (c *Client) GrantRoutinePrivilege(ctx context.Context, privilege string, schema string, routineName string, user string) error {
	routineType, err := c.GetRoutineType(ctx, schema, routineName)
	if err != nil {
		return err
	}

	schemaEsc, err := escapeMySQLIdent(schema)
	if err != nil {
		return err
	}
	routineNameEsc, err := escapeMySQLIdent(routineName)
	if err != nil {
		return err
	}

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

	query := fmt.Sprintf("GRANT %s ON %s %s.%s TO %s",
		privilege, strings.ToUpper(routineType), schemaEsc, routineNameEsc, userGrant)

	_ = c.db.MustExec(query)
	return nil
}

func (c *Client) RevokeRoutinePrivilege(ctx context.Context, privilege string, schema string, routineName string, user string) error {
	routineType, err := c.GetRoutineType(ctx, schema, routineName)
	if err != nil {
		return err
	}

	schemaEsc, err := escapeMySQLIdent(schema)
	if err != nil {
		return err
	}
	routineNameEsc, err := escapeMySQLIdent(routineName)
	if err != nil {
		return err
	}

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

	query := fmt.Sprintf("REVOKE %s ON %s %s.%s FROM %s",
		privilege, strings.ToUpper(routineType), schemaEsc, routineNameEsc, userRevoke)
	_ = c.db.MustExec(query)
	return nil
}

func (c *Client) GetRoutineType(ctx context.Context, schema, routineName string) (string, error) {
	query := `
		SELECT ROUTINE_TYPE
		FROM information_schema.ROUTINES
		WHERE ROUTINE_SCHEMA = ? AND ROUTINE_NAME = ?
	`
	var routineType string
	err := c.db.QueryRowContext(ctx, query, schema, routineName).Scan(&routineType)
	if err != nil {
		return "", fmt.Errorf("failed to get routine type: %w", err)
	}
	return routineType, nil
}
