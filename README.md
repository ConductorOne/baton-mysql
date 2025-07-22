![Baton Logo](./docs/images/baton-logo.png)

# `baton-mysql` [![Go Reference](https://pkg.go.dev/badge/github.com/conductorone/baton-mysql.svg)](https://pkg.go.dev/github.com/conductorone/baton-mysql) ![main ci](https://github.com/conductorone/baton-mysql/actions/workflows/main.yaml/badge.svg)

`baton-mysql` is a connector for MySQL 5.7 and 8.\* built using the [Baton SDK](https://github.com/conductorone/baton-sdk). It connects to your MySQL cluster and syncs privilege information about what access is granted to various users and roles.

Check out [Baton](https://github.com/conductorone/baton) to learn more about the project in general.

# Getting Started

## brew

```
brew install conductorone/baton/baton conductorone/baton/baton-mysql

baton-mysql --connection-string "baton:baton-password@tcp(127.0.0.1:3306)/"
baton resources
```

## docker

```
docker run --rm -v $(pwd):/out ghcr.io/conductorone/baton-mysql:latest -f "/out/sync.c1z" --connection-string "baton:baton-password@tcp(127.0.0.1:3306)/"
docker run --rm -v $(pwd):/out ghcr.io/conductorone/baton:latest -f "/out/sync.c1z" resources
```

## source

```
go install github.com/conductorone/baton/cmd/baton@main
go install github.com/conductorone/baton-mysql/cmd/baton-mysql@main

baton-mysql --connection-string "baton:baton-password@tcp(127.0.0.1:3306)/"
baton resources
```

# Data Model

`baton-mysql` will sync information about the following MySQL resources:

- Uses
- Roles
- Servers
- Routines
- Tables
- Columns
- Databases

By default, the connector will introspect all databases that it has access to read. While some of these databases are informational, write access to `mysql` means that users can grant their own access, so it is important to include in reviews. You can use the `--skip-database` flag or the `BATON_SKIP_DATABASE` environment variable to exclude specific databases from being synced. The following internal databases are included by default:

- `performance_schema`
- `information_schema`
- `mysql`
- `sys`

# Advanced Setup

1. Create a new user for the connector to connect to MySQL as. Be sure to create and save the secure password for this user:

```mysql
CREATE USER baton IDENTIFIED BY 'secure-password';
```

2. Grant your new role the privileges required by the connector for inspecting privileges.
   MySQL 5.7:

```mysql
GRANT SELECT (Host, User, Db, Select_priv, Insert_priv, Update_priv,  Delete_priv, Create_priv, Drop_priv,
              Grant_priv, References_priv, Index_priv, Alter_priv, Create_tmp_table_priv, Lock_tables_priv,
              Execute_priv, Create_view_priv, Show_view_priv, Create_routine_priv,
              Alter_routine_priv, Event_priv, Trigger_priv) ON mysql.db TO conductorone;
GRANT SELECT (Host, User, Db, Table_priv, Table_name) ON mysql.tables_priv TO conductorone;
GRANT SELECT (Host, User, Db, Column_name, Column_priv, Table_name) ON mysql.columns_priv TO conductorone;
GRANT SELECT (Host, User, Select_priv, Insert_priv, Update_priv,  Delete_priv, Create_priv, Drop_priv, Reload_priv,
              Shutdown_priv, Process_priv,
              References_priv, Index_priv, Alter_priv, Show_db_priv, Super_priv, Create_tmp_table_priv, Lock_tables_priv,
              Execute_priv, Repl_slave_priv, Repl_client_priv, Create_view_priv, Show_view_priv, Create_routine_priv,
              Alter_routine_priv, Create_user_priv, Event_priv, Trigger_priv, Create_tablespace_priv,
              File_priv, Grant_priv, authentication_string) ON mysql.user TO conductorone;
```

MySQL 8+:

```mysql
GRANT SELECT (USER, HOST, PRIV, WITH_GRANT_OPTION) ON mysql.global_grants TO conductorone;
GRANT SELECT (Host, User, Db, Select_priv, Insert_priv, Update_priv,  Delete_priv, Create_priv, Drop_priv,
              Grant_priv, References_priv, Index_priv, Alter_priv, Create_tmp_table_priv, Lock_tables_priv,
              Execute_priv, Create_view_priv, Show_view_priv, Create_routine_priv,
              Alter_routine_priv, Event_priv, Trigger_priv) ON mysql.db TO conductorone;
GRANT SELECT (Host, User, Db, Table_priv, Table_name) ON mysql.tables_priv TO conductorone;
GRANT SELECT (Host, User, Db, Column_name, Column_priv, Table_name) ON mysql.columns_priv TO conductorone;
GRANT SELECT (Host, User, Select_priv, Insert_priv, Update_priv,  Delete_priv, Create_priv, Drop_priv, Reload_priv,
              Shutdown_priv, Process_priv,
              References_priv, Index_priv, Alter_priv, Show_db_priv, Super_priv, Create_tmp_table_priv, Lock_tables_priv,
              Execute_priv, Repl_slave_priv, Repl_client_priv, Create_view_priv, Show_view_priv, Create_routine_priv,
              Alter_routine_priv, Create_user_priv, Event_priv, Trigger_priv, Create_tablespace_priv, Create_role_priv,
              Drop_role_priv, File_priv, Grant_priv, authentication_string) ON mysql.user TO conductorone;
GRANT SELECT (FROM_HOST, FROM_USER, TO_HOST, TO_USER, WITH_ADMIN_OPTION) ON mysql.role_edges TO conductorone;
```

3. Grant your new user SELECT on each of the databases that you would like the connector to scan. In all likelihood, you will want this to be all databases. The connector does not look at any data within the databases, but `SELECT` is required in order to introspect the various schemas.

```mysql
GRANT SELECT ON *.* TO baton;
```

# Contributing, Support and Issues

We started Baton because we were tired of taking screenshots and manually building spreadsheets. We welcome contributions, and ideas, no matter how small -- our goal is to make identity and permissions sprawl less painful for everyone. If you have questions, problems, or ideas: Please open a Github Issue!

See [CONTRIBUTING.md](https://github.com/ConductorOne/baton/blob/main/CONTRIBUTING.md) for more details.

# `baton-mysql` Command Line Usage

```
baton-mysql

Usage:
  baton-mysql [flags]
  baton-mysql [command]

Available Commands:
  capabilities       Get connector capabilities
  completion         Generate the autocompletion script for the specified shell
  help               Help about any command

Flags:
      --client-id string           The client ID used to authenticate with ConductorOne ($BATON_CLIENT_ID)
      --client-secret string       The client secret used to authenticate with ConductorOne ($BATON_CLIENT_SECRET)
      --collapse-users             Combine user@host pairs into a single user@[hosts...] identity $(BATON_COLLAPSE_USERS)
      --connection-string string   The connection string for connecting to MySQL ($BATON_CONNECTION_STRING)
      --expand-columns strings     Provide a table like db.table to expand the column privileges into their own entitlements. $(BATON_EXPAND_COLUMNS)
  -f, --file string                The path to the c1z file to sync with ($BATON_FILE) (default "sync.c1z")
  -h, --help                       help for baton-mysql
      --log-format string          The output format for logs: json, console ($BATON_LOG_FORMAT) (default "json")
      --log-level string           The log level: debug, info, warn, error ($BATON_LOG_LEVEL) (default "info")
  -p, --provisioning               This must be set in order for provisioning actions to be enabled. ($BATON_PROVISIONING)
      --skip-database strings      Skip syncing privileges from these databases ($BATON_SKIP_DATABASE)
  -v, --version                    version for baton-mysql

Use "baton-mysql [command] --help" for more information about a command.
```
