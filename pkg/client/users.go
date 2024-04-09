package client

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"go.uber.org/zap"
)

const (
	UserType = "user"
	RoleType = "role"
)

type User struct {
	UserType string `db:"user_type"`
	Host     string `db:"Host"`
	User     string `db:"User"`
	Privs    string `db:"privs"`
}

func (u *User) GetID() string {
	return fmt.Sprintf("%s:%s@%s", u.UserType, u.User, u.Host)
}

func (u *User) GetPrivs(ctx context.Context) map[string]struct{} {
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

func (c *Client) userPrivsSelect(ctx context.Context, sb *strings.Builder) error {
	_, err := sb.WriteString(`CONCAT(
CASE WHEN Select_priv = 'Y' THEN 'select,' ELSE '' END,
CASE WHEN Insert_priv = 'Y' THEN 'insert,' ELSE '' END,
CASE WHEN Update_priv = 'Y' THEN 'update,' ELSE '' END,
CASE WHEN Delete_priv = 'Y' THEN 'delete,' ELSE '' END,
CASE WHEN Create_priv = 'Y' THEN 'create,' ELSE '' END,
CASE WHEN Drop_priv = 'Y' THEN 'drop,' ELSE '' END,
CASE WHEN Reload_priv = 'Y' THEN 'reload,' ELSE '' END,
CASE WHEN Shutdown_priv = 'Y' THEN 'shutdown,' ELSE '' END,
CASE WHEN Process_priv = 'Y' THEN 'process,' ELSE '' END,
CASE WHEN File_priv = 'Y' THEN 'file,' ELSE '' END,
CASE WHEN Grant_priv = 'Y' THEN 'grant,' ELSE '' END,
CASE WHEN References_priv = 'Y' THEN 'references,' ELSE '' END,
CASE WHEN Index_priv = 'Y' THEN 'index,' ELSE '' END,
CASE WHEN Alter_priv = 'Y' THEN 'alter,' ELSE '' END,
CASE WHEN Show_db_priv = 'Y' THEN 'show_databases,' ELSE '' END,
CASE WHEN Create_tmp_table_priv = 'Y' THEN 'create_temporary_tables,' ELSE '' END,
CASE WHEN Lock_tables_priv = 'Y' THEN 'lock_tables,' ELSE '' END,
CASE WHEN Execute_priv = 'Y' THEN 'execute,' ELSE '' END,
CASE WHEN Repl_slave_priv = 'Y' THEN 'replication_slave,' ELSE '' END,
CASE WHEN Repl_client_priv = 'Y' THEN 'replication_client,' ELSE '' END,
CASE WHEN Create_view_priv = 'Y' THEN 'create_view,' ELSE '' END,
CASE WHEN Show_view_priv = 'Y' THEN 'show_view,' ELSE '' END,
CASE WHEN Create_routine_priv = 'Y' THEN 'create_routine,' ELSE '' END,
CASE WHEN Alter_routine_priv = 'Y' THEN 'alter_routine,' ELSE '' END,
CASE WHEN Create_user_priv = 'Y' THEN 'create_user,' ELSE '' END,
CASE WHEN Event_priv = 'Y' THEN 'event,' ELSE '' END,
CASE WHEN Trigger_priv = 'Y' THEN 'trigger,' ELSE '' END,
CASE WHEN Create_tablespace_priv = 'Y' THEN 'create_tablespace,' ELSE '' END`)
	if err != nil {
		return err
	}

	if c.IsVersion8() {
		_, err = sb.WriteString(`,
CASE WHEN Create_role_priv = 'Y' THEN 'create_role,' ELSE '' END,
CASE WHEN Drop_role_priv = 'Y' THEN 'drop_role,' ELSE '' END
`)
		if err != nil {
			return err
		}
	}

	_, err = sb.WriteString(") as privs,")
	if err != nil {
		return err
	}
	return nil
}

// GetUser returns a single user@host row and its perms
// Grants required:
//
//	GRANT SELECT (Host, User, Select_priv, Insert_priv, Update_priv,  Delete_priv, Create_priv, Drop_priv, Reload_priv,
//				  References_priv, Index_priv, Alter_priv, Show_db_priv, Super_priv, Create_tmp_table_priv, Lock_tables_priv,
//				  Execute_priv, Repl_slave_priv, Repl_client_priv, Create_view_priv, Show_view_priv, Create_routine_priv,
//				  Alter_routine_priv, Create_user_priv, Event_priv, Trigger_priv, Create_tablespace_priv, Create_role_priv,
//				  Drop_role_priv, File_priv,, Grant_priv, authentication_string) ON mysql.user TO user@host;
func (c *Client) GetUser(ctx context.Context, user string, host string) (*User, error) {
	u := User{}
	sb := &strings.Builder{}
	_, err := sb.WriteString(`SELECT User, Host,`)
	if err != nil {
		return nil, err
	}

	err = c.userPrivsSelect(ctx, sb)
	if err != nil {
		return nil, err
	}
	_, err = sb.WriteString(`CASE WHEN authentication_string = '' THEN 'role' ELSE 'user' END AS user_type `)
	if err != nil {
		return nil, err
	}
	_, err = sb.WriteString(`FROM mysql.user WHERE User = ? AND Host = ?`)
	if err != nil {
		return nil, err
	}

	err = c.db.GetContext(ctx, &u, sb.String(), user, host)
	if err != nil {
		return nil, err
	}

	return &u, nil
}

func (c *Client) getUserGroupedByHostQuery(ctx context.Context) (*strings.Builder, error) {
	sb := &strings.Builder{}
	_, err := sb.WriteString(`SELECT User, GROUP_CONCAT(Host) as Host, 'user' AS user_type FROM mysql.user `)
	return sb, err
}

func (c *Client) getUsersQuery(ctx context.Context) (*strings.Builder, error) {
	sb := &strings.Builder{}
	_, err := sb.WriteString(`SELECT Host, User, CASE WHEN authentication_string = '' THEN 'role' ELSE 'user' END AS user_type FROM mysql.user `)
	return sb, err
}

// ListUsers queries the server and fetches all the users for the given  page.
func (c *Client) ListUsers(ctx context.Context, userType string, pager *Pager, collapseUsers bool) ([]*User, string, error) {
	l := ctxzap.Extract(ctx)
	l.Debug("listing users", zap.String("user_type", userType))

	offset, limit, err := pager.Parse()
	if err != nil {
		return nil, "", err
	}
	var args []interface{}

	sb, err := c.getUsersQuery(ctx)
	if err != nil {
		return nil, "", err
	}
	if collapseUsers {
		sb, err = c.getUserGroupedByHostQuery(ctx)
		if err != nil {
			return nil, "", err
		}
	}

	switch userType {
	case UserType:
		_, err = sb.WriteString(`WHERE authentication_string != '' `)
		if err != nil {
			return nil, "", err
		}

	case RoleType:
		_, err = sb.WriteString(`WHERE authentication_string = '' `)
		if err != nil {
			return nil, "", err
		}

	default:
		return nil, "", fmt.Errorf("unexpected user type %s", userType)
	}

	if collapseUsers {
		_, err = sb.WriteString(`GROUP BY User `)
		if err != nil {
			return nil, "", err
		}
	}

	_, err = sb.WriteString("LIMIT ? ")
	if err != nil {
		return nil, "", err
	}
	args = append(args, limit+1)
	if offset > 0 {
		_, err = sb.WriteString("OFFSET ?")
		if err != nil {
			return nil, "", err
		}
		args = append(args, offset)
	}
	var ret []*User
	err = c.db.SelectContext(ctx, &ret, sb.String(), args...)
	if err != nil {
		return nil, "", err
	}

	var nextPageToken string
	if len(ret) > limit {
		offset += limit
		nextPageToken = strconv.Itoa(offset)
		ret = ret[:limit]
	}

	return ret, nextPageToken, nil
}
