package migrations

import (
	"context"
	"hepatitis-antiviral/cli"
	"strconv"

	"github.com/jackc/pgx/v4/pgxpool"
)

func Migrate(ctx context.Context, pool *pgxpool.Pool) {
	for i, m := range miglist {
		cli.NotifyMsg("info", "Running migration ["+strconv.Itoa(i)+"/"+strconv.Itoa(len(miglist))+"] "+m.name)
		m.fn(ctx, pool)
	}
}
