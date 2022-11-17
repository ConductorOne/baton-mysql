package connector

import (
	"github.com/conductorone/baton-mysql/pkg/client"
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
)

// Resource Types.
var (
	resourceTypeServer = &v2.ResourceType{
		Id:          client.ServerType,
		DisplayName: titleCaser.String(client.ServerType),
	}
	resourceTypeTable = &v2.ResourceType{
		Id:          client.TableType,
		DisplayName: titleCaser.String(client.TableType),
	}
	resourceTypeDatabase = &v2.ResourceType{
		Id:          client.DatabaseType,
		DisplayName: titleCaser.String(client.DatabaseType),
	}
	resourceTypeColumn = &v2.ResourceType{
		Id:          client.ColumnType,
		DisplayName: titleCaser.String(client.ColumnType),
	}

	resourceTypeRoutine = &v2.ResourceType{
		Id:          client.RoutineType,
		DisplayName: titleCaser.String(client.RoutineType),
	}
	resourceTypeUser = &v2.ResourceType{
		Id:          client.UserType,
		DisplayName: titleCaser.String(client.UserType),
		Traits:      []v2.ResourceType_Trait{v2.ResourceType_TRAIT_USER},
	}
	resourceTypeRole = &v2.ResourceType{
		Id:          client.RoleType,
		DisplayName: titleCaser.String(client.RoleType),
	}

	allResourceTypes = []*v2.ResourceType{
		resourceTypeServer,
		resourceTypeTable,
		resourceTypeDatabase,
		resourceTypeColumn,
		resourceTypeRoutine,
		resourceTypeUser,
		resourceTypeRole,
	}

	globalDatabaseTableColumnScope = []*v2.ResourceType{
		resourceTypeServer,
		resourceTypeDatabase,
		resourceTypeTable,
		resourceTypeColumn,
	}

	globalDatabaseTableScope = []*v2.ResourceType{
		resourceTypeServer,
		resourceTypeDatabase,
		resourceTypeTable,
	}

	globalDatabaseScope = []*v2.ResourceType{
		resourceTypeServer,
		resourceTypeDatabase,
	}

	globalScope = []*v2.ResourceType{
		resourceTypeServer,
	}
)
