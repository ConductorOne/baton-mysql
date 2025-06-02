package main

import (
	"context"
	"fmt"
	"os"

	"github.com/conductorone/baton-mysql/pkg/connector"
	"github.com/conductorone/baton-sdk/pkg/config"
	"github.com/conductorone/baton-sdk/pkg/connectorbuilder"
	"github.com/conductorone/baton-sdk/pkg/field"
	sdkTypes "github.com/conductorone/baton-sdk/pkg/types"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

var version = "dev"

func main() {
	ctx := context.Background()

	_, cmd, err := config.DefineConfiguration(
		ctx,
		"baton-mysql",
		getConnector,
		field.Configuration{
			Fields: ConfigurationFields,
		},
	)
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

func getConnector(ctx context.Context, v *viper.Viper) (sdkTypes.ConnectorServer, error) {
	l := ctxzap.Extract(ctx)
	if err := ValidateConfig(v); err != nil {
		return nil, err
	}

	cb, err := connector.New(ctx, v.GetString(ConnectionString.FieldName), v.GetStringSlice(SkipDatabases.FieldName), v.GetStringSlice(ExpandColumns.FieldName), v.GetBool(CollapseUsers.FieldName))
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
