package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"

	"goravel/app/facades"
)

type M20260814000003CreateCommentsTable struct{}

func (r *M20260814000003CreateCommentsTable) Signature() string {
	return "20260814000003_create_comments_table"
}

func (r *M20260814000003CreateCommentsTable) Up() error {
	if !facades.Schema().HasTable("comments") {
		if err := facades.Schema().Create("comments", func(table schema.Blueprint) {
			table.ID()
			table.UnsignedBigInteger("post_id")
			table.UnsignedBigInteger("user_id")
			table.Text("content")
			table.Timestamps()
			table.Index("post_id")
			table.Index("user_id")
		}); err != nil {
			return err
		}
	}
	return nil
}

func (r *M20260814000003CreateCommentsTable) Down() error {
	return facades.Schema().DropIfExists("comments")
}
