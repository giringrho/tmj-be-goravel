package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"

	"goravel/app/facades"
)

type M20260814000002CreatePostsTable struct{}

func (r *M20260814000002CreatePostsTable) Signature() string {
	return "20260814000002_create_posts_table"
}

func (r *M20260814000002CreatePostsTable) Up() error {
	if !facades.Schema().HasTable("posts") {
		if err := facades.Schema().Create("posts", func(table schema.Blueprint) {
			table.ID()
			table.UnsignedBigInteger("user_id")
			table.String("title", 255)
			table.Text("content")
			table.Boolean("published").Default(false)
			table.Timestamps()
			table.Index("user_id")
			table.Index("published")
		}); err != nil {
			return err
		}
	}
	return nil
}

func (r *M20260814000002CreatePostsTable) Down() error {
	return facades.Schema().DropIfExists("posts")
}
