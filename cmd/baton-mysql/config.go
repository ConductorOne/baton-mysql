package main

import (
	"github.com/conductorone/baton-sdk/pkg/field"
	"github.com/spf13/viper"
)

var (
	ConnectionString = field.StringField(
		"connection-string",
		field.WithDescription("The connection string for connecting to MySQL ($BATON_CONNECTION_STRING)"),
		field.WithRequired(true),
	)
	SkipDatabases = field.StringSliceField(
		"skip-database",
		field.WithDescription("Skip syncing privileges from these databases ($BATON_SKIP_DATABASE)"),
		field.WithRequired(false),
	)
	ExpandColumns = field.StringSliceField(
		"expand-columns",
		field.WithDescription("Provide a table like db.table to expand the column privileges into their own entitlements. $(BATON_EXPAND_COLUMNS)"),
		field.WithRequired(false),
	)
	CollapseUsers = field.BoolField(
		"collapse-users",
		field.WithDescription("Combine user@host pairs into a single user@[hosts...] identity $(BATON_COLLAPSE_USERS)"),
		field.WithDefaultValue(false),
		field.WithRequired(false),
	)
	// ConfigurationFields defines the external configuration required for the
	// connector to run. Note: these fields can be marked as optional or
	// required.
	ConfigurationFields = []field.SchemaField{ConnectionString, SkipDatabases, ExpandColumns, CollapseUsers}
)

// validateConfig is run after the configuration is loaded, and should return an error if it isn't valid.
func ValidateConfig(v *viper.Viper) error {
	return nil
}
