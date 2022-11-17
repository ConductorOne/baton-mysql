# baton-mysql

## Usage
```
baton-mysql

Usage:
  baton-mysql [flags]
  baton-mysql [command]

Available Commands:
  completion         Generate the autocompletion script for the specified shell
  help               Help about any command

Flags:
      --collapse-users             Combine user@host pairs into a single user@[hosts...] identity
      --connection-string string   The connection string for connecting to MySQL
      --expand-columns strings     Provide a table like db.table to expand the column privileges into their own entitlements.
  -f, --file string                The path to the c1z file to sync with ($C1_FILE) (default "sync.c1z")
  -h, --help                       help for baton-mysql
      --log-format string          The output format for logs: json, console ($C1_LOG_FORMAT) (default "json")
      --log-level string           The log level: debug, info, warn, error ($C1_LOG_LEVEL) (default "info")
      --skip-database strings      Skip syncing privileges from these databases
  -v, --version                    version for baton-mysql

Use "baton-mysql [command] --help" for more information about a command.
```

## Setup
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
GRANT SELECT ON *.* TO conductorone;
```

## Tips
1. All config options can be specified via the environment: `--connection-string` => `C1_CONNECTION_STRING`
2. You will need to specify a connection string that informs the connector how to connect to your MySQL instance.
    - An example: `"baton:password@tcp(127.0.0.1:3306)/"`
    - `baton` is the MySQL user to connect as
    - `password` is the password for the MySQL user
    - `127.0.0.1:3306` is the hostname and port to connect to
3. By default, the connector will introspect all databases that it has access to read. While some of these databases are informational, write access to `mysql` means that users can grant their own access, so it is important to include in reviews. You can use the `--skip-database` flag or the `C1_SKIP_DATABASE` environment variable to exclude specific databases from being synced. The following internal databases are included by default:
   - `performance_schema`
   - `information_schema`
   - `mysql`
   - `sys`
