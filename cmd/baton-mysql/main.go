package main

import (
	"context"
	"fmt"
	"os"

	"github.com/conductorone/baton-mysql/pkg/connector"
	"github.com/conductorone/baton-sdk/pkg/cli"
	"github.com/conductorone/baton-sdk/pkg/connectorbuilder"
	sdkTypes "github.com/conductorone/baton-sdk/pkg/types"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"go.uber.org/zap"
)

var version = "dev"

func main() {
	ctx := context.Background()

	cfg := &config{}
	cmd, err := cli.NewCmd(ctx, "baton-mysql", cfg, validateConfig, getConnector)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	cmd.Version = version
	cmd.PersistentFlags().String("connection-string", "", "The connection string for connecting to MySQL ($BATON_CONNECTION_STRING)")
	cmd.PersistentFlags().StringSlice("skip-database", nil, "Skip syncing privileges from these databases ($BATON_SKIP_DATABASE)")
	cmd.PersistentFlags().StringSlice(
		"expand-columns",
		nil,
		`Provide a table like db.table to expand the column privileges into their own entitlements. $(BATON_EXPAND_COLUMNS)`,
	)
	cmd.PersistentFlags().Bool("collapse-users", false, "Combine user@host pairs into a single user@[hosts...] identity $(BATON_COLLAPSE_USERS)")
	err = cmd.Execute()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}

func getConnector(ctx context.Context, cfg *config) (sdkTypes.ConnectorServer, error) {
	l := ctxzap.Extract(ctx)

	cb, err := connector.New(ctx, cfg.ConnectionString, cfg.SkipDatabases, cfg.ExpandColumns, cfg.CollapseUsers)
	if err != nil {
		l.Error("error creating connector builder", zap.Error(err))
		return nil, err
	}

	c, err := connectorbuilder.NewConnector(ctx, cb)
	if err != nil {
		l.Error("error creating connector from connector builder", zap.Error(err))
		return nil, err
	}

	return c, nil
}
