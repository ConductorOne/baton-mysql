package client

import (
	"context"
	"fmt"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

var supportedReleases = []string{
	"8", "5.7", "10.",
}

type dbResourceID struct {
	ResourceTypeID  string
	DatabaseName    string
	ResourceName    string
	SubResourceName string
}

func (t dbResourceID) Database() dbResourceID {
	return dbResourceID{
		ResourceTypeID: DatabaseType,
		DatabaseName:   t.DatabaseName,
	}
}

func (t dbResourceID) Table() dbResourceID {
	return dbResourceID{
		ResourceTypeID: TableType,
		DatabaseName:   t.DatabaseName,
		ResourceName:   t.ResourceName,
	}
}

func (t dbResourceID) Column() dbResourceID {
	return dbResourceID{
		ResourceTypeID:  ColumnType,
		DatabaseName:    t.DatabaseName,
		ResourceName:    t.ResourceName,
		SubResourceName: t.SubResourceName,
	}
}

func (t dbResourceID) SQLString() (string, error) {
	var sb strings.Builder
	_, err := sb.WriteString(fmt.Sprintf("`%s`", t.DatabaseName))
	if err != nil {
		return "", err
	}
	if t.ResourceName != "" && (t.ResourceTypeID == TableType || t.ResourceTypeID == ColumnType) {
		_, err = sb.WriteString(fmt.Sprintf(".`%s`", t.ResourceName))
		if err != nil {
			return "", err
		}
	}

	return sb.String(), nil
}

func (t dbResourceID) String() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s:%s", t.ResourceTypeID, t.DatabaseName)) //nolint:revive // too much work to fix

	if t.ResourceName != "" {
		sb.WriteString(".")            //nolint:revive // too much work to fix
		sb.WriteString(t.ResourceName) //nolint:revive // too much work to fix

		if t.SubResourceName != "" {
			sb.WriteString(".")               //nolint:revive // too much work to fix
			sb.WriteString(t.SubResourceName) //nolint:revive // too much work to fix
		}
	}

	return sb.String()
}

func newDbResourceID(in string) (dbResourceID, error) {
	if in == "" {
		return dbResourceID{}, fmt.Errorf("cannot use empty string to make db resource ID")
	}
	inParts := strings.SplitN(in, ":", 2)
	if len(inParts) != 2 {
		return dbResourceID{}, fmt.Errorf("resource ID must have a type prefix")
	}

	dri := dbResourceID{
		ResourceTypeID: inParts[0],
	}

	parts := strings.Split(inParts[1], ".")

	switch len(parts) {
	case 1:
		dri.DatabaseName = parts[0]
	case 2:
		dri.DatabaseName = parts[0]
		dri.ResourceName = parts[1]
	case 3:
		dri.DatabaseName = parts[0]
		dri.ResourceName = parts[1]
		dri.SubResourceName = parts[2]
	default:
		return dbResourceID{}, fmt.Errorf("invalid db resource ID: %s", in)
	}

	return dri, nil
}

type Client struct {
	db      *sqlx.DB
	version string
}

func (c *Client) IsVersion8() bool {
	return strings.HasPrefix(c.version, "8.")
}

func (c *Client) ValidateConnection(ctx context.Context) error {
	var v int
	err := c.db.GetContext(ctx, &v, "SELECT 1;")
	if err != nil {
		return err
	}

	return nil
}

func New(ctx context.Context, dsn string) (*Client, error) {
	db, err := sqlx.Connect("mysql", dsn)
	if err != nil {
		return nil, err
	}

	db.SetConnMaxLifetime(time.Minute * 1)
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	c := &Client{
		db: db,
	}

	si, err := c.GetServerInfo(ctx)
	if err != nil {
		return nil, err
	}

	supported := false
	for _, sv := range supportedReleases {
		if strings.HasPrefix(si.Version, sv) {
			supported = true
			break
		}
	}

	if !supported {
		return nil, fmt.Errorf("%s is not a supported version of MySQL", c.version)
	}

	c.version = si.Version

	return c, nil
}
