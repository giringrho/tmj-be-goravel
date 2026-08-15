package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"

	"goravel/app/facades"
)

type M20260814000001CreateUsersTable struct{}

func (r *M20260814000001CreateUsersTable) Signature() string {
	return "20260814000001_create_users_table"
}

func (r *M20260814000001CreateUsersTable) Up() error {
	if !facades.Schema().HasTable("users") {
		if err := facades.Schema().Create("users", func(table schema.Blueprint) {
			table.ID()
			table.String("name", 100)
			table.String("email", 255)
			table.String("password", 255)
			table.String("role", 20).Default("user")
			table.Timestamps()
			table.Unique("email")
		}); err != nil {
			return err
		}
	}
	return nil
}

func (r *M20260814000001CreateUsersTable) Down() error {
	return facades.Schema().DropIfExists("users")
}
