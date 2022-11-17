package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/conductorone/baton-sdk/pkg/cli"
)

// config defines the external configuration required for the connector to run.
type config struct {
	cli.BaseConfig `mapstructure:",squash"` // Puts the base config options in the same place as the connector options

	ConnectionString string   `mapstructure:"connection-string"`
	SkipDatabases    []string `mapstructure:"skip-database"`
	ExpandColumns    []string `mapstructure:"expand-columns"`
	CollapseUsers    bool     `mapstructure:"collapse-users"`
}

// validateConfig is run after the configuration is loaded, and should return an error if it isn't valid.
func validateConfig(ctx context.Context, cfg *config) error {
	if cfg.ConnectionString == "" {
		return fmt.Errorf("--connection-string is required")
	}

	for _, col := range cfg.ExpandColumns {
		p := strings.Split(col, ".")
		if len(p) != 2 {
			return fmt.Errorf("malformed expand-columns option. Must be in the format of db.table")
		}
	}

	return nil
}
