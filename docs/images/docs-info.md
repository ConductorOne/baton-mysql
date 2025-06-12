While developing the connector, please fill out this form. This information is needed to write docs and to help other users set up the connector.

## Connector capabilities

1. What resources does the connector sync?

    -Users
    -Tables
    -Servers
    -Routines
    -Roles
    -Databases
    -Columns(optional)

2. Can the connector provision any resources? If so, which ones? 

    -Account provision and deprovision
    -Entitlement provision(Grant & Revoke) for all others resources


## Connector credentials 

1. What credentials or information are needed to set up the connector? (For example, API key, client ID and secret, domain, etc.)

    -   --connection-string string   The connection string for connecting to MySQL, for example: "baton:baton-password@tcp(127.0.0.1:3306)/"
    -   --expand-columns strings     Provide a table like db.table to expand the column privileges into their own entitlements. This is optional,
     for example: baton_db.empleados

2. For each item in the list above: 

   * How does a user create or look up that credential or info? Please include links to (non-gated) documentation, screenshots (of the UI or of gated docs), or a video of the process. 

   -Customers can create a dedicated MySQL user with appropriate privileges using the following SQL:
    CREATE USER 'baton'@'%' IDENTIFIED BY 'baton-password';  
    GRANT SELECT ON *.* TO 'baton'@'%'; -- for sync only
    GRANT ALL PRIVILEGES ON *.* TO 'baton'@'%'; -- for sync and provision

    https://dev.mysql.com/doc/refman/8.0/en/create-user.html
    https://dev.mysql.com/doc/refman/8.0/en/grant.html


   * Does the credential need any specific scopes or permissions? If so, list them here. 

   -Yes:

    For sync: SELECT privileges on mysql.*, information_schema.*, and all databases/tables you want to sync.

    For provisioning: GRANT OPTION and ALL PRIVILEGES (or at least GRANT, SELECT, INSERT, UPDATE, DELETE, REFERENCES) on the target resources.

    * If applicable: Is the list of scopes or permissions different to sync (read) versus provision (read-write)? If so, list the difference here. 

    -| Purpose   | Required Privileges                                           |
     | --------- | ------------------------------------------------------------- |
     | Sync      | `SELECT` on `information_schema`, `mysql`, target DBs         |
     | Provision | `GRANT OPTION`, `CREATE USER`, `DROP USER`, `GRANT`, `REVOKE` |


     * What level of access or permissions does the user need in order to create the credentials? (For example, must be a super administrator, must have access to the admin console, etc.)  

     -The credential must be created by a MySQL user with CREATE USER and GRANT OPTION privileges — typically a DBA or superuser (root or similar).

