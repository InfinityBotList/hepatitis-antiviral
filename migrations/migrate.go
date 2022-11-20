package migrations

import (
	"context"
	"hepatitis-antiviral/cli"
	"strconv"

	"github.com/jackc/pgx/v4/pgxpool"
)

func Migrate(ctx context.Context, pool *pgxpool.Pool) {
	cli.StartBar("migrations", int64(len(miglist))+1, true)
	for i, m := range miglist {
		cli.Bar.Increment()
		cli.NotifyMsg("info", "Running migration ["+strconv.Itoa(i)+"/"+strconv.Itoa(len(miglist))+"] "+m.name)

		m.fn(ctx, pool)
	}

	cli.Bar.Increment()
}
