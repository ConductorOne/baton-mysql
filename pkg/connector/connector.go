package connector

import (
	"context"
	"io"

	"github.com/conductorone/baton-mysql/pkg/client"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/connectorbuilder"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var titleCaser = cases.Title(language.English)

// connectorImpl implements the ConnectorServer interface for syncing with a MySQL server.
type connectorImpl struct {
	client        *client.Client
	skipDbs       map[string]struct{}
	expandCols    map[string]struct{}
	collapseUsers bool
}

// Metadata returns metadata about the connector. This currently includes the hostname for the server.
func (c *connectorImpl) Metadata(ctx context.Context) (*v2.ConnectorMetadata, error) {
	sm, err := c.client.GetServerInfo(ctx)
	if err != nil {
		return nil, err
	}

	return &v2.ConnectorMetadata{
		DisplayName: sm.Name,
	}, nil
}

// Validate the connection to the MySQL service.
func (c *connectorImpl) Validate(ctx context.Context) (annotations.Annotations, error) {
	err := c.client.ValidateConnection(ctx)
	if err != nil {
		return nil, err
	}

	return nil, nil
}

// Asset returns the content-type and a ReadCloser for fetching the asset
// The MySQL connector doesn't emit any assets.
func (c *connectorImpl) Asset(ctx context.Context, asset *v2.AssetRef) (string, io.ReadCloser, error) {
	return "", nil, nil
}

func (c *connectorImpl) ResourceSyncers(ctx context.Context) []connectorbuilder.ResourceSyncer {
	syncers := []connectorbuilder.ResourceSyncer{
		newServerSyncer(ctx, c.client),
		newDatabaseSyncer(ctx, c.client, c.skipDbs),
		newTableSyncer(ctx, c.client, c.expandCols),
		newRoutineSyncer(ctx, c.client),
		newUserSyncer(ctx, c.client, c.skipDbs, c.expandCols, c.collapseUsers),
	}

	if c.client.IsVersion8() {
		syncers = append(syncers, newRoleSyncer(ctx, c.client, c.skipDbs, c.expandCols))
	}

	if len(c.expandCols) > 0 {
		syncers = append(syncers, newColumnSyncer(ctx, c.client, c.expandCols))
	}

	return syncers
}

// New returns a new MySQL connector.
func New(ctx context.Context, dsn string, skipDbs []string, expandColumns []string, collapseUsers bool) (*connectorImpl, error) {
	c, err := client.New(ctx, dsn)
	if err != nil {
		return nil, err
	}

	dbs := make(map[string]struct{})
	expandCols := make(map[string]struct{})
	for _, db := range skipDbs {
		dbs[db] = struct{}{}
	}
	for _, table := range expandColumns {
		expandCols[table] = struct{}{}
	}
	return &connectorImpl{
		client:        c,
		skipDbs:       dbs,
		expandCols:    expandCols,
		collapseUsers: collapseUsers,
	}, nil
}
