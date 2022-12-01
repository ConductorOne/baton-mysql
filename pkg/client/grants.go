package client

import (
	"context"
	"strings"

	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"go.uber.org/zap"
)

type GlobalGrant struct {
	User      string `db:"USER"`
	Host      string `db:"HOST"`
	Priv      string `db:"PRIV"`
	WithGrant string `db:"WITH_GRANT_OPTION"`
}

// ListGlobalGrants returns the set of grants from the mysql.global_grants
// Required MySQL grant for connector:
//
//	GRANT SELECT (USER, HOST, PRIV, WITH_GRANT_OPTION) ON mysql.global_grants TO user@host;
func (c *Client) ListGlobalGrants(ctx context.Context, user string, host string) ([]*GlobalGrant, error) {
	l := ctxzap.Extract(ctx)
	l.Debug("checking global grants")

	q := `SELECT USER, HOST, PRIV, WITH_GRANT_OPTION FROM mysql.global_grants WHERE USER=? AND HOST=?`

	var ret []*GlobalGrant
	err := c.db.SelectContext(ctx, &ret, q, user, host)
	if err != nil {
		return nil, err
	}

	return ret, nil
}

type DatabaseGrant struct {
	Id       string `db:"-"`
	User     string `db:"User"`
	Host     string `db:"Host"`
	Database string `db:"Db"`
	Privs    string `db:"privs"`
}

func (u *DatabaseGrant) GetPrivs(ctx context.Context) map[string]struct{} {
	ret := make(map[string]struct{})

	privs := strings.Split(u.Privs, ",")
	for _, p := range privs {
		priv := strings.TrimSpace(p)
		if priv == "" {
			continue
		}

		ret[priv] = struct{}{}
	}

	return ret
}

// ListDatabaseGrants returns a single user@host row and its perms
// Grants required:
//
//	GRANT SELECT (Host, User, Db, Select_priv, Insert_priv, Update_priv,  Delete_priv, Create_priv, Drop_priv,
//				  Grant_priv, References_priv, Index_priv, Alter_priv, Create_tmp_table_priv, Lock_tables_priv,
//				  Execute_priv, Create_view_priv, Show_view_priv, Create_routine_priv,
//				  Alter_routine_priv, Event_priv, Trigger_priv) ON mysql.db TO user@host;
func (c *Client) ListDatabaseGrants(ctx context.Context, user string, host string) ([]*DatabaseGrant, error) {
	q := `SELECT
    		User,
    		Host,
    		Db,
    		CONCAT(
              CASE WHEN Select_priv = 'Y' THEN 'select,' ELSE '' END,
              CASE WHEN Insert_priv = 'Y' THEN 'insert,' ELSE '' END,
              CASE WHEN Update_priv = 'Y' THEN 'update,' ELSE '' END,
              CASE WHEN Delete_priv = 'Y' THEN 'delete,' ELSE '' END,
              CASE WHEN Create_priv = 'Y' THEN 'create,' ELSE '' END,
              CASE WHEN Drop_priv = 'Y' THEN 'drop,' ELSE '' END,
              CASE WHEN Grant_priv = 'Y' THEN 'grant,' ELSE '' END,
              CASE WHEN References_priv = 'Y' THEN 'references,' ELSE '' END,
              CASE WHEN Index_priv = 'Y' THEN 'index,' ELSE '' END,
              CASE WHEN Alter_priv = 'Y' THEN 'alter,' ELSE '' END,
              CASE WHEN Create_tmp_table_priv = 'Y' THEN 'create_temporary_tables,' ELSE '' END,
              CASE WHEN Lock_tables_priv = 'Y' THEN 'lock_tables,' ELSE '' END,
              CASE WHEN Execute_priv = 'Y' THEN 'execute,' ELSE '' END,
              CASE WHEN Create_view_priv = 'Y' THEN 'create_view,' ELSE '' END,
              CASE WHEN Show_view_priv = 'Y' THEN 'show_view,' ELSE '' END,
              CASE WHEN Create_routine_priv = 'Y' THEN 'create_routine,' ELSE '' END,
              CASE WHEN Alter_routine_priv = 'Y' THEN 'alter_routine,' ELSE '' END,
              CASE WHEN Event_priv = 'Y' THEN 'event,' ELSE '' END,
              CASE WHEN Trigger_priv = 'Y' THEN 'trigger,' ELSE '' END
            ) AS privs
		FROM mysql.db WHERE User = ? AND Host = ?`

	var ret []*DatabaseGrant
	err := c.db.SelectContext(ctx, &ret, q, user, host)
	if err != nil {
		return nil, err
	}

	for i, r := range ret {
		ret[i].Id = dbResourceID{
			ResourceTypeID: DatabaseType,
			DatabaseName:   r.Database,
		}.String()
	}

	return ret, nil
}

type TableGrant struct {
	Id       string `db:"-"`
	User     string `db:"User"`
	Host     string `db:"Host"`
	Database string `db:"Db"`
	Table    string `db:"Table_name"`
	Privs    string `db:"Table_priv"`
}

func (u *TableGrant) GetPrivs(ctx context.Context) map[string]struct{} {
	ret := make(map[string]struct{})

	privs := strings.Split(u.Privs, ",")
	for _, p := range privs {
		priv := strings.ReplaceAll(strings.TrimSpace(p), " ", "_")
		if priv == "" {
			continue
		}

		ret[priv] = struct{}{}
	}

	return ret
}

// ListTableGrants returns a single user@host row and its perms
// Grants required:
//
//	GRANT SELECT (Host, User, Db, Table_priv, Table_name) ON mysql.tables_priv TO user@host;
func (c *Client) ListTableGrants(ctx context.Context, user string, host string) ([]*TableGrant, error) {
	q := `SELECT
    		User,
    		Host,
    		Db,
    		Table_name,
    		Table_priv
		FROM mysql.tables_priv WHERE User = ? AND Host = ?`

	var ret []*TableGrant
	err := c.db.SelectContext(ctx, &ret, q, user, host)
	if err != nil {
		return nil, err
	}

	for i, r := range ret {
		ret[i].Id = dbResourceID{
			ResourceTypeID: TableType,
			DatabaseName:   r.Database,
			ResourceName:   r.Table,
		}.String()
	}

	return ret, nil
}

type ColumnGrant struct {
	Id       string `db:"-"`
	User     string `db:"User"`
	Host     string `db:"Host"`
	Database string `db:"Db"`
	Table    string `db:"Table_name"`
	Column   string `db:"Column_name"`
	Privs    string `db:"Column_priv"`
}

func (u *ColumnGrant) TableID() string {
	return dbResourceID{
		ResourceTypeID: TableType,
		DatabaseName:   u.Database,
		ResourceName:   u.Table,
	}.String()
}

// GetPrivs parses the columns grant data from mysql.
func (u *ColumnGrant) GetPrivs(ctx context.Context) map[string]struct{} {
	ret := make(map[string]struct{})

	privs := strings.Split(u.Privs, ",")
	for _, p := range privs {
		priv := strings.ReplaceAll(strings.TrimSpace(p), " ", "_")
		if priv == "" {
			continue
		}

		ret[priv] = struct{}{}
	}

	return ret
}

// ListColumnGrants returns a single user@host row and its perms
// Grants required:
//
//	GRANT SELECT (Host, User, Db, Column_name, Column_priv, Table_name) ON mysql.columns_priv TO user@host;
func (c *Client) ListColumnGrants(ctx context.Context, user string, host string) ([]*ColumnGrant, error) {
	q := `SELECT
    		User,
    		Host,
    		Db,
    		Table_name,
    		Column_name,
    		Column_priv
		FROM mysql.columns_priv WHERE User = ? AND Host = ?`

	var ret []*ColumnGrant
	err := c.db.SelectContext(ctx, &ret, q, user, host)
	if err != nil {
		return nil, err
	}

	for i, r := range ret {
		ret[i].Id = dbResourceID{
			ResourceTypeID:  ColumnType,
			DatabaseName:    r.Database,
			ResourceName:    r.Table,
			SubResourceName: r.Column,
		}.String()
	}

	return ret, nil
}

type ProxyGrant struct {
	Id          string `db:"-"`
	User        string `db:"User"`
	Host        string `db:"Host"`
	ProxiedHost string `db:"Proxied_host"`
	ProxiedUser string `db:"Proxied_user"`
	WithGrant   int    `db:"With_grant"`
}

// ListProxyGrants returns a single user@host row and its perms
// Grants required:
//
//	GRANT SELECT (Host, User, Db, Column_name, Column_priv, Table_name) ON mysql.columns_priv TO user@host;
func (c *Client) ListProxyGrants(ctx context.Context, user string, host string) ([]*ProxyGrant, error) {
	q := `SELECT
    		User,
    		Host,
    		Proxied_user,
    		Proxied_host,
    		With_grant
		FROM mysql.proxies_priv WHERE User = ? AND Host = ?`

	var out []*ProxyGrant
	err := c.db.SelectContext(ctx, &out, q, user, host)
	if err != nil {
		return nil, err
	}

	var ret []*ProxyGrant
	for _, r := range out {
		if r.ProxiedUser == r.User {
			continue
		}

		newR := r

		if r.ProxiedUser == "" && r.ProxiedHost == "" {
			s, err := c.GetServerInfo(ctx)
			if err != nil {
				return nil, err
			}
			newR.Id = s.ID
			ret = append(ret, newR)
			continue
		}
		u, err := c.GetUser(ctx, r.ProxiedUser, r.ProxiedHost)
		if err != nil {
			return nil, err
		}
		newR.Id = u.GetID()
		ret = append(ret, newR)
	}

	return ret, nil
}

type RoleGrant struct {
	Id        string `db:"-"`
	FromHost  string `db:"FROM_HOST"`
	FromUser  string `db:"FROM_USER"`
	ToHost    string `db:"TO_HOST"`
	ToUser    string `db:"TO_USER"`
	WithGrant string `db:"WITH_ADMIN_OPTION"`
}

// ListRoleGrants returns a single user@host row and its role edges
// Grants required:
//
//	GRANT SELECT (FROM_HOST, FROM_USER, TO_HOST, TO_USER, WITH_ADMIN_OPTION) ON mysql.role_edges TO user@host;
func (c *Client) ListRoleGrants(ctx context.Context, user string, host string) ([]*RoleGrant, error) {
	q := `SELECT
			FROM_HOST,
			FROM_USER,
			TO_HOST,
			TO_USER,
			WITH_ADMIN_OPTION
		FROM mysql.role_edges WHERE FROM_USER = ? AND FROM_HOST = ?`

	var out []*RoleGrant
	err := c.db.SelectContext(ctx, &out, q, user, host)
	if err != nil {
		return nil, err
	}

	var ret []*RoleGrant
	for _, r := range out {
		if r.FromUser == r.ToUser {
			continue
		}

		newR := r

		u, err := c.GetUser(ctx, r.ToUser, r.ToHost)
		if err != nil {
			ctxzap.Extract(ctx).Error(
				"unable to fetch to user for role. Ignoring grant",
				zap.Error(err),
				zap.String("to_user", r.ToUser),
				zap.String("to_host", r.ToHost),
			)
			continue
		}
		newR.Id = u.GetID()
		ret = append(ret, newR)
	}

	return ret, nil
}
