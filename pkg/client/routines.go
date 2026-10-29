package client

import (
	"context"
	"database/sql"
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

	userGrant, err := quoteAccounts(user)
	if err != nil {
		return err
	}

	// R2 fallback form: the primary `GRANT GRANT OPTION ON PROCEDURE ...` statement could
	// not be live-verified on MySQL 8.0 in this environment (no MySQL validation
	// environment available), so the connector ships the `WITH GRANT OPTION` form
	// (the extractServerPrivilege WITH GRANT OPTION precedent in pkg/connector/server.go).
	var query string
	if privilege == "GRANT OPTION" {
		query = fmt.Sprintf("GRANT USAGE ON %s %s.%s TO %s WITH GRANT OPTION",
			strings.ToUpper(routineType), schemaEsc, routineNameEsc, userGrant)
	} else {
		query = fmt.Sprintf("GRANT %s ON %s %s.%s TO %s",
			privilege, strings.ToUpper(routineType), schemaEsc, routineNameEsc, userGrant)
	}

	_, err = c.db.ExecContext(ctx, query)
	return err
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

	userRevoke, err := quoteAccounts(user)
	if err != nil {
		return err
	}

	query := fmt.Sprintf("REVOKE %s ON %s %s.%s FROM %s",
		privilege, strings.ToUpper(routineType), schemaEsc, routineNameEsc, userRevoke)
	_, err = c.db.ExecContext(ctx, query)
	return err
}

// RevokeRoutineGrantOption revokes the grant option for the given held privileges on a routine.
// It builds `REVOKE GRANT OPTION FOR <heldPrivileges> ON <TYPE> <schema>.<routine> FROM <user>`.
func (c *Client) RevokeRoutineGrantOption(ctx context.Context, heldPrivileges string, schema string, routineName string, user string) error {
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

	userRevoke, err := quoteAccounts(user)
	if err != nil {
		return err
	}

	query := fmt.Sprintf("REVOKE GRANT OPTION FOR %s ON %s %s.%s FROM %s",
		heldPrivileges, strings.ToUpper(routineType), schemaEsc, routineNameEsc, userRevoke)
	_, err = c.db.ExecContext(ctx, query)
	return err
}

func (c *Client) GetRoutineType(ctx context.Context, schema, routineName string) (string, error) {
	query := `
		SELECT ROUTINE_TYPE
		FROM information_schema.ROUTINES
		WHERE ROUTINE_SCHEMA = ? AND ROUTINE_NAME = ?
	`

	rows, err := c.db.QueryxContext(ctx, query, schema, routineName)
	if err != nil {
		return "", fmt.Errorf("baton-mysql: failed to query routine type for %s.%s: %w", schema, routineName, err)
	}
	defer rows.Close()

	var types []string
	for rows.Next() {
		var routineType string
		if err := rows.Scan(&routineType); err != nil {
			return "", fmt.Errorf("baton-mysql: failed to scan routine type for %s.%s: %w", schema, routineName, err)
		}
		types = append(types, routineType)
	}
	if rows.Err() != nil {
		return "", fmt.Errorf("baton-mysql: failed to iterate routine type rows for %s.%s: %w", schema, routineName, rows.Err())
	}

	return routineTypesFromRows(types, schema, routineName)
}

// routineTypesFromRows resolves the routine type from the collected ROUTINE_TYPE rows.
// 0 rows -> not found; 1 distinct type -> return it; >1 distinct type -> ambiguity error.
func routineTypesFromRows(rows []string, schema, routineName string) (string, error) {
	if len(rows) == 0 {
		return "", fmt.Errorf("baton-mysql: routine %s.%s not found: %w", schema, routineName, sql.ErrNoRows)
	}

	seen := make(map[string]struct{}, len(rows))
	for _, r := range rows {
		seen[r] = struct{}{}
	}
	if len(seen) > 1 {
		return "", fmt.Errorf("baton-mysql: ambiguous routine name %s.%s resolves to both PROCEDURE and FUNCTION; routine resource IDs do not encode routine type — refusing to grant/revoke the wrong object", schema, routineName)
	}

	for r := range seen {
		return r, nil
	}
	return "", fmt.Errorf("baton-mysql: routine %s.%s not found: %w", schema, routineName, sql.ErrNoRows)
}
