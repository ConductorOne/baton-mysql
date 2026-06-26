# Baton MySQL - Connector Documentation

This document provides information needed to set up and use the connector.

## Connector Capabilities

### 1. What resources does the connector sync?

| Resource | Description |
|----------|-------------|
| **Server** | The MySQL server instance itself; root of the resource hierarchy |
| **Database** | Individual databases hosted on the server; scoped privilege assignments |
| **Table** | Tables within databases; table-level privilege grants |
| **Routine** | Stored procedures and functions; execute/alter privilege grants |
| **User** | MySQL accounts (`user@host`); principal identities in the access graph |
| **Role** | Named role objects (MySQL 8+ only); role-to-user assignment grants |
| **Column** | Individual table columns (optional, requires `--expand-columns`); column-level privilege grants |

### 2. Can the connector provision any resources? If so, which ones?

Yes.

| Resource | Grant | Revoke | Create | Delete |
|----------|-------|--------|--------|--------|
| **Database privileges** | ✅ Grants a privilege on a database to a user | ✅ Revokes a privilege on a database from a user | - | - |
| **Table privileges** | ✅ Grants a privilege on a table to a user | ✅ Revokes a privilege on a table from a user | - | - |
| **Routine privileges** | ✅ Grants a privilege on a routine to a user | ✅ Revokes a privilege on a routine from a user | - | - |
| **Role assignments** | ✅ Assigns a role to a user (MySQL 8+ only) | ✅ Removes a role from a user (MySQL 8+ only) | - | - |
| **User accounts** | - | - | ✅ Creates a new MySQL user with a generated password | ✅ Drops a MySQL user account |

## Connector Credentials

### 1. What credentials or information are needed to set up the connector?

| Credential | Required | Description |
|------------|----------|-------------|
| **connection-string** | Yes | MySQL DSN in Go driver format: `user:password@tcp(host:port)/` |
| **skip-database** | No | Comma-separated list of database names to exclude from sync |
| **expand-columns** | No | Tables (`db.table`) whose column-level privileges are expanded into individual entitlements |
| **collapse-users** | No | Combines `user@host` accounts with the same username into a single identity (default: false) |

### 2. How are these credentials obtained?

Create a dedicated MySQL user and grant it the minimum read privileges on `mysql` system tables. See `README.md` (Advanced Setup section) for the exact `GRANT` statements for MySQL 5.7 and MySQL 8+.

The connection string format is: `username:password@tcp(hostname:3306)/`

## Additional Notes

### MySQL Version Requirements

- **MySQL 5.7 and 8.x** are supported.
- Role sync and role grant/revoke are available on **MySQL 8+** only. The connector detects the server version at runtime and enables role support automatically.

### API Documentation Links

- [MySQL 8.0 Reference Manual](https://dev.mysql.com/doc/refman/8.0/en/)
- [MySQL 5.7 Reference Manual](https://dev.mysql.com/doc/refman/5.7/en/)
- [Go MySQL Driver DSN format](https://github.com/go-sql-driver/mysql#dsn-data-source-name)
