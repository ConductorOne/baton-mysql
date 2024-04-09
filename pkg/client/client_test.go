package client

import (
	"context"
	"fmt"
	"math/rand"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

var hostPool = []string{
	"localhost",
	"127.0.0.1",
	"::1",
	"%",
	"%.example.com",
	"198.51.100.%",
	"198.51.100.0/255.255.255.0",
	"198.51.0.0/255.255.0.0",
}

func Test_newDbResourceID(t *testing.T) {
	type args struct {
		in string
	}
	tests := []struct {
		name    string
		args    args
		want    dbResourceID
		wantErr bool
	}{
		{
			name: "database",
			args: args{"db:db"},
			want: dbResourceID{
				ResourceTypeID: "db",
				DatabaseName:   "db",
			},
		},
		{
			name: "table",
			args: args{"table:db.table"},
			want: dbResourceID{
				ResourceTypeID: "table",
				DatabaseName:   "db",
				ResourceName:   "table",
			},
		},
		{
			name: "column",
			args: args{"column:db.table.column"},
			want: dbResourceID{
				ResourceTypeID:  "column",
				DatabaseName:    "db",
				ResourceName:    "table",
				SubResourceName: "column",
			},
		},
		{
			name:    "no prefix",
			args:    args{"dev"},
			want:    dbResourceID{},
			wantErr: true,
		},
		{
			name:    "empty",
			args:    args{""},
			want:    dbResourceID{},
			wantErr: true,
		},
		{
			name:    "malformed",
			args:    args{"prefix:some.random.string.that.contains.many.dots"},
			want:    dbResourceID{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := newDbResourceID(tt.args.in)
			if (err != nil) != tt.wantErr {
				t.Errorf("newDbResourceID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("newDbResourceID() got = %v, want %v", got, tt.want)
			}
		})
	}
}

type grantItem struct {
	resourceType string
	resourceIDs  []dbResourceID
}

func newGrantItem(rType string, resources ...dbResourceID) grantItem {
	return grantItem{
		resourceType: rType,
		resourceIDs:  resources,
	}
}

func genUsersAndRoles(userCount int, roleCount int) (map[string]struct{}, map[string]struct{}) {
	users := make(map[string]struct{})
	roles := make(map[string]struct{})

	collisions := 0

	// Run until we have enough users, or we collide 3 times. Success resets the counter.
	for collisions < 3 && len(users) < userCount {
		hostName := hostPool[rand.Intn(len(hostPool))]
		accountName := fmt.Sprintf(`'user_%d'@'%s'`, rand.Intn(userCount/2), hostName)
		if _, ok := users[accountName]; !ok {
			users[accountName] = struct{}{}
			collisions = 0
			continue
		}
		collisions++
	}

	collisions = 0
	// Run until we have enough roles, or we collide 3 times.
	for collisions < 3 && len(roles) < roleCount {
		accountName := fmt.Sprintf("'role_%d'", rand.Intn(9999))
		if _, ok := roles[accountName]; !ok {
			roles[accountName] = struct{}{}
			collisions = 0
			continue
		}
		collisions++
	}

	return users, roles
}

func Test_generateRandomGrants(t *testing.T) {
	t.Skip()
	rand.Seed(time.Now().UnixNano())
	ctx := context.Background()
	databases := make(map[string]dbResourceID)
	tables := make(map[string]dbResourceID)
	columns := make(map[string]dbResourceID)

	dsn := "root:password@tcp(127.0.0.1:3306)/"
	c, err := New(ctx, dsn)
	require.NoError(t, err)

	// Generate random users and roles to grant privileges to
	users, roles := genUsersAndRoles(10, 5)

	// Scan the database and collect resources to grant privileges to
	dbName := "dev"
	var cols []*ColumnModel
	err = c.db.SelectContext(ctx, &cols, "SELECT TABLE_SCHEMA, TABLE_NAME, COLUMN_NAME FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=?", dbName)
	require.NoError(t, err)

	for _, c := range cols {
		colID := dbResourceID{
			ResourceTypeID:  ColumnType,
			DatabaseName:    c.Database,
			ResourceName:    c.Table,
			SubResourceName: c.Name,
		}

		if _, ok := columns[colID.String()]; !ok {
			columns[colID.String()] = colID
		}
		if _, ok := tables[colID.Table().String()]; !ok {
			tables[colID.Table().String()] = colID.Table()
		}
		if _, ok := databases[colID.Database().String()]; !ok {
			databases[colID.Database().String()] = colID.Database()
		}
	}

	var grantStatements strings.Builder
	var cleanupStatements strings.Builder

	grantCounts := 100
	var grantItems []grantItem
	for i := 0; i < grantCounts; i++ {
		rCol := randomItem(columns)

		switch rand.Intn(3) {
		case 0:
			// Pick column's database
			grantItems = append(grantItems, newGrantItem(DatabaseType, databases[rCol.Database().String()]))
		case 1:
			// Pick column's table
			grantItems = append(grantItems, newGrantItem(TableType, tables[rCol.Table().String()]))
		case 2: // randomly pick some columns from the same table
			var tCols []dbResourceID

			relatedColPrefix := strings.Replace(rCol.Table().String(), TableType, ColumnType, 1)
			// gather columns for table
			for k, v := range columns {
				if strings.HasPrefix(k, relatedColPrefix+".") {
					tCols = append(tCols, v)
				}
			}

			if len(tCols) == 0 {
				continue
			}

			// pick how many -- at most half of the columns
			pickedCount := rand.Intn(5)

			// Randomly pick pickedCount, making sure we don't duplicate
			var pickedCols []dbResourceID
		Outer:
			for i := 0; i < pickedCount; i++ {
				idx := rand.Intn(len(tCols))
				for _, x := range pickedCols {
					if x == tCols[idx] {
						continue Outer
					}
				}
				pickedCols = append(pickedCols, tCols[idx])
			}

			grantItems = append(grantItems, newGrantItem(ColumnType, pickedCols...))
		}
	}

	privCount := 6

	_, _ = grantStatements.WriteString("-- Users\n")
	_, _ = cleanupStatements.WriteString("\n-- Users\n")
	for u := range users {
		// create users
		_, _ = grantStatements.WriteString("CREATE USER ")
		_, _ = grantStatements.WriteString(u)
		_, _ = grantStatements.WriteString(" IDENTIFIED BY RANDOM PASSWORD;\n")

		// drop users
		_, _ = cleanupStatements.WriteString("DROP USER ")
		_, _ = cleanupStatements.WriteString(u)
		_, _ = cleanupStatements.WriteString(";\n")
	}

	_, _ = grantStatements.WriteString("\n-- Roles\n")
	_, _ = cleanupStatements.WriteString("\n-- Roles\n")
	for r := range roles {
		_, _ = grantStatements.WriteString("CREATE ROLE ")
		_, _ = grantStatements.WriteString(r)
		_, _ = grantStatements.WriteString(";\n")

		// drop roles
		_, _ = cleanupStatements.WriteString("DROP ROLE ")
		_, _ = cleanupStatements.WriteString(r)
		_, _ = cleanupStatements.WriteString(";\n")
	}

	_, _ = grantStatements.WriteString("\n-- Grants\n")
	for _, g := range grantItems {
		// pick a user a role
		var accountID string
		switch rand.Intn(2) {
		case 0:
			accountID = randomKey(users)
		case 1:
			accountID = randomKey(roles)
		}

		if len(g.resourceIDs) == 0 {
			continue
		}

		switch g.resourceType {
		case ColumnType:
			privs := randomPrivs(columnPrivs(), privCount)

			if len(privs) == 0 {
				continue
			}

			_, _ = grantStatements.WriteString("GRANT ")
			for ii, p := range privs {
				if ii != 0 {
					_, _ = grantStatements.WriteString(", ")
				}

				_, _ = grantStatements.WriteString(fmt.Sprintf(`%s (`, p))
				for jj, col := range g.resourceIDs {
					if jj != 0 {
						_, _ = grantStatements.WriteString(", ")
					}
					_, _ = grantStatements.WriteString(fmt.Sprintf("`%s`", col.SubResourceName))
				}
				_, _ = grantStatements.WriteString(")")
			}
			resourceString, err := g.resourceIDs[0].SQLString()
			require.NoError(t, err)
			_, _ = grantStatements.WriteString(fmt.Sprintf(" ON %s TO %s;", resourceString, accountID))
			_ = grantStatements.WriteByte('\n')

		case TableType:
			privs := randomPrivs(tablePrivs(), privCount)

			if len(privs) == 0 {
				continue
			}

			_, _ = grantStatements.WriteString("GRANT")
			for ii, p := range privs {
				if ii != 0 {
					_, _ = grantStatements.WriteString(",")
				}
				_, _ = grantStatements.WriteString(" ")

				_, _ = grantStatements.WriteString(p)
			}
			resourceString, err := g.resourceIDs[0].SQLString()
			require.NoError(t, err)
			_, _ = grantStatements.WriteString(fmt.Sprintf(" ON %s TO %s;\n", resourceString, accountID))

		case DatabaseType:
			privs := randomPrivs(dbPrivs(), privCount)

			if len(privs) == 0 {
				continue
			}

			_, _ = grantStatements.WriteString("GRANT")
			for ii, p := range privs {
				if ii != 0 {
					_, _ = grantStatements.WriteString(",")
				}
				_, _ = grantStatements.WriteString(" ")

				_, _ = grantStatements.WriteString(p)
			}
			resourceString, err := g.resourceIDs[0].SQLString()
			require.NoError(t, err)
			_, _ = grantStatements.WriteString(fmt.Sprintf(" ON %s.* TO %s;\n", resourceString, accountID))
		default:
			require.NoError(t, fmt.Errorf("invalid resource type for grant item"))
		}
	}

	fmt.Printf("Grants\n%s\n", grantStatements.String())
	fmt.Printf("Cleanup\n%s\n", cleanupStatements.String())
}

func randomPrivs(privs []string, n int) []string {
	var ret []string
	if len(privs) <= n {
		return privs
	}

	collisions := 0
Outer:
	for collisions < 3 && len(ret) < n {
		idx := rand.Intn(len(privs))

		for _, r := range ret {
			if privs[idx] == r {
				collisions++
				continue Outer
			}
		}

		ret = append(ret, privs[idx])
		collisions = 0
	}

	return ret
}

func columnPrivs() []string {
	return []string{"INSERT", "REFERENCES", "SELECT", "UPDATE"}
}

func tablePrivs() []string {
	return append(columnPrivs(),
		"ALTER",
		"CREATE",
		"CREATE VIEW",
		"DELETE",
		"DROP",
		"GRANT OPTION",
		"SHOW VIEW",
		"TRIGGER",
	)
}

func dbPrivs() []string {
	return append(tablePrivs(),
		"ALTER ROUTINE",
		"CREATE ROUTINE",
		"CREATE TEMPORARY TABLES",
		"EVENT",
		"EXECUTE",
		"LOCK TABLES",
	)
}

// Privs for users, routines, and global to finish filling out test support
// func routinePrivs() []string {
// 	return []string{
// 		"ALTER ROUTINE",
// 		"EXECUTE",
// 		"GRANT OPTION",
// 	}
// }
//

// func userPrivs() []string {
// 	return []string{"PROXY"}
// }
//
// func global() []string {
// 	return append(dbPrivs(),
// 		"CREATE ROLE",
// 		"CREATE TABLESPACE",
// 		"CREATE USER",
// 		"DROP ROLE",
// 		"FILE",
// 		"PROCESS",
// 		"RELOAD",
// 		"REPLICATION CLIENT",
// 		"REPLICATION SLAVE",
// 		"SHOW DATABASES",
// 		"SHUTDOWN",
// 		"SUPER",
// 		"APPLICATION_PASSWORD_ADMIN",
// 		"AUDIT_ABORT_EXEMPT",
// 		"AUDIT_ADMIN",
// 		"AUTHENTICATION_POLICY_ADMIN",
// 		"BACKUP_ADMIN",
// 		"BINLOG_ADMIN",
// 		"BINLOG_ENCRYPTION_ADMIN",
// 		"CLONE_ADMIN",
// 		"CONNECTION_ADMIN",
// 		"ENCRYPTION_KEY_ADMIN",
// 		"FIREWALL_ADMIN",
// 		"FIREWALL_EXEMPT",
// 		"FIREWALL_USER",
// 		"FLUSH_OPTIMIZER_COSTS",
// 		"FLUSH_STATUS",
// 		"FLUSH_TABLES",
// 		"FLUSH_USER_RESOURCES",
// 		"GROUP_REPLICATION_ADMIN",
// 		"INNODB_REDO_LOG_",
// 		"INNODB_REDO_LOG_ARCHIVE",
// 		"NDB_STORED_USER",
// 		"PASSWORDLESS_USER_ADMIN",
// 		"PERSIST_RO_VARIABLES_ADMIN",
// 		"REPLICATION_APPLIER",
// 		"REPLICATION_SLAVE_ADMIN",
// 		"RESOURCE_GROUP_ADMIN",
// 		"RESOURCE_GROUP_USER",
// 		"ROLE_ADMIN",
// 		"SESSION_VARIABLES_ADMIN",
// 		"SET_USER_ID",
// 		"SHOW_ROUTINE",
// 		"SKIP_QUERY_REWRITE",
// 		"SYSTEM_USER",
// 		"SYSTEM_VARIABLES_ADMIN",
// 		"TABLE_ENCRYPTION_ADMIN",
// 		"VERSION_TOKEN_ADMIN",
// 		"XA_RECOVER_ADMIN",
// 	)
// }

func randomItem(items map[string]dbResourceID) dbResourceID {
	for _, dri := range items {
		return dri
	}

	panic("empty map")
}

func randomKey(items map[string]struct{}) string {
	for k := range items {
		return k
	}

	panic("empty map")
}
