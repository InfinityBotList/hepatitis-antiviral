// Put your migration functions here
package migrations

import (
	"context"

	"github.com/jackc/pgx/v4/pgxpool"
)

var miglist = []migrator{
	{
		name: "remove certified+claimed and replace with type",
		fn: func(ctx context.Context, pool *pgxpool.Pool) {
			_, err := pool.Exec(ctx, "UPDATE bots SET type = 'claimed' WHERE claimed = true")

			if err != nil {
				panic(err)
			}

			_, err = pool.Exec(ctx, "UPDATE bots SET type = 'certified' WHERE certified = true")

			if err != nil {
				panic(err)
			}

			_, err = pool.Exec(ctx, "ALTER TABLE bots DROP COLUMN claimed, DROP COLUMN certified")

			if err != nil {
				panic(err)
			}
		},
	},
	{
		name: "fixup claimed_by",
		fn: func(ctx context.Context, pool *pgxpool.Pool) {
			_, err := pool.Exec(ctx, "UPDATE bots SET claimed_by = NULL WHERE claimed_by = '' OR claimed_by = 'none'")

			if err != nil {
				panic(err)
			}
		},
	},
}
