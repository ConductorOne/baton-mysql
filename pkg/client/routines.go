package client

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/go-sql-driver/mysql"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
)

const RoutineType = "routine"

// ErrRoutineGrantNotFound reports that the account held no privilege on the routine.
var ErrRoutineGrantNotFound = errors.New("no such routine grant")

// ErrRoutineNotFound reports that the connector account can see no routine under that
// name in the schema. It does not report that the routine is gone; see GetRoutineType.
var ErrRoutineNotFound = errors.New("no such routine")

// Named as the MySQL error reference names it, so the number can be checked against it.
const erNonexistingProcGrant = 1403

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

	userGrant, err := quoteAccounts(user)
	if err != nil {
		return err
	}

	query := fmt.Sprintf("GRANT %s ON %s %s.%s TO %s",
		privilege, strings.ToUpper(routineType), schemaEsc, routineNameEsc, userGrant)

	_, err = c.db.ExecContext(ctx, query)
	return err
}

// RevokeRoutinePrivilege removes a routine privilege from the account. It revokes one host
// at a time so that a host holding nothing can be told apart from the whole account
// holding nothing, which is what ErrRoutineGrantNotFound reports.
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

	userName, hosts, err := SplitUserHost(user)
	if err != nil {
		return err
	}

	hostList := strings.Split(hosts, ",")
	notFound := 0
	for _, host := range hostList {
		account, err := quoteUserHost(userName, host)
		if err != nil {
			return err
		}

		query := fmt.Sprintf("REVOKE %s ON %s %s.%s FROM %s",
			privilege, strings.ToUpper(routineType), schemaEsc, routineNameEsc, account)
		_, err = c.db.ExecContext(ctx, query)

		var mysqlErr *mysql.MySQLError
		switch {
		case errors.As(err, &mysqlErr) && mysqlErr.Number == erNonexistingProcGrant:
			notFound++
		case err != nil:
			return err
		}
	}

	if notFound == len(hostList) {
		return fmt.Errorf("%w: %s %s.%s for %s", ErrRoutineGrantNotFound, privilege, schema, routineName, user)
	}

	return nil
}

// GetRoutineType resolves whether a routine is a PROCEDURE or a FUNCTION, which the GRANT
// and REVOKE statements need and the routine resource ID does not carry.
//
// The query returns a row only for a routine the connector account holds a privilege on,
// so no rows means the account cannot see the routine. That covers a routine that was
// dropped and a routine the account simply cannot reach, and the two are indistinguishable
// here. ErrRoutineNotFound is therefore an error for the caller to report, never a reason
// to answer that a privilege on it is already revoked.
func (c *Client) GetRoutineType(ctx context.Context, schema, routineName string) (string, error) {
	query := `
		SELECT DISTINCT ROUTINE_TYPE
		FROM information_schema.ROUTINES
		WHERE ROUTINE_SCHEMA = ? AND ROUTINE_NAME = ?
	`

	var routineTypes []string
	err := c.db.SelectContext(ctx, &routineTypes, query, schema, routineName)
	if err != nil {
		return "", fmt.Errorf("get routine type for %s.%s: %w", schema, routineName, err)
	}

	switch len(routineTypes) {
	case 0:
		return "", fmt.Errorf("%w: %s.%s", ErrRoutineNotFound, schema, routineName)
	case 1:
		return routineTypes[0], nil
	default:
		return "", fmt.Errorf(
			"%s.%s is both a procedure and a function, so the routine to act on is ambiguous: %s",
			schema, routineName, strings.Join(routineTypes, ", "),
		)
	}
}
