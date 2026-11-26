# Baton MySQL - Connector Documentation

This document provides information needed to set up and use the connector.

## Connector Capabilities

### 1. What resources does the connector sync?

| Resource | Description |
|----------|-------------|
| **Server** | The MySQL server instance itself; root of the resource hierarchy |
| **Database** | Individual databases hosted on the server; scoped privilege assignments |
| **Table** | Tables within databases; table-level privilege grants |
| **Routine** | Stored procedures and functions; `EXECUTE`, `ALTER ROUTINE` and `GRANT OPTION` privilege grants (grants require `--sync-routine-grants`) |
| **User** | MySQL accounts (`user@host`); principal identities in the access graph |
| **Role** | Named role objects (MySQL 8+ only); role-to-user assignment grants |
| **Column** | Individual table columns (optional, requires `--expand-columns`); column-level privilege grants |

### 2. Can the connector provision any resources? If so, which ones?

Yes.

| Resource | Grant | Revoke | Create | Delete |
|----------|-------|--------|--------|--------|
| **Database privileges** | ✅ Grants a privilege on a database to a user | ✅ Revokes a privilege on a database from a user | - | - |
| **Table privileges** | ✅ Grants a privilege on a table to a user | ✅ Revokes a privilege on a table from a user | - | - |
| **Routine privileges** | ✅ Grants a privilege on a routine to a user or role | ✅ Revokes a privilege on a routine from a user or role | - | - |
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

### Routine Privilege Grants

Who holds a privilege on a stored procedure or function is read from `mysql.procs_priv`,
which the connector queries only with `--sync-routine-grants`. An account that holds
`GRANT SELECT ON *.*`, which the Advanced Setup instructions grant, already reads the
table; a column grant on it is needed only for an account scoped more narrowly than that.
The flag is off by default so that an upgrade changes nothing for an account the read
would fail on. With it on, a denied read fails the sync and names the `GRANT` to run.

The flag gates grant and revoke as well as the sync. With it off the connector refuses to
change a routine privilege, because a privilege it granted would be absent from the next
sync, ConductorOne would delete the grant, and the privilege would stay in MySQL with no
way to revoke it from ConductorOne.

Turning the flag off after it has been on has the same effect in reverse: the next sync
reports no routine grants, and ConductorOne reads their absence as a revocation of every
one of them. The routine resources and their entitlements survive either way, since the
flag gates only the read.

### Routines That Share a Name

MySQL allows a schema to hold a stored procedure and a stored function under the same
name, and grants privileges on each separately. A routine resource is identified by its
schema and name, so both routines answer to one resource. Privileges on such a routine are
left out of the sync, and a grant or revoke against it returns an error naming the
ambiguity rather than acting on one of the two. Routines whose name is unique within their
schema, which is the usual case, are unaffected.

### API Documentation Links

- [MySQL 8.0 Reference Manual](https://dev.mysql.com/doc/refman/8.0/en/)
- [MySQL 5.7 Reference Manual](https://dev.mysql.com/doc/refman/5.7/en/)
- [Go MySQL Driver DSN format](https://github.com/go-sql-driver/mysql#dsn-data-source-name)
